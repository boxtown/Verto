package session

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"github.com/boxtown/verto"
	"github.com/boxtown/verto/plugins"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// ErrCipherTooShort is returned by DecryptCookie
// when performing decryption and the cipher text is too short
// to be valid
var ErrCipherTooShort = errors.New("Cipher text is too short")

// ErrBadHMAC is returned by DecryptCookie when the HMAC read from
// the cookie does not match the HMAC calculated from its contents
var ErrBadHMAC = errors.New("Mis-matched HMAC")

// ErrMissingKey is returned by NewSecureCookie and DecryptCookie
// if the hashKey parameter is missing
var ErrMissingKey = errors.New("Missing required hashKey argument")

// SESSIONKEY is the constant name used to denote both the verto
// session cookie and the session injection
const SESSIONKEY = "_VertoSession"

// Plugin is a plugin that instantiates a relevant
// session instance per request based on the SessionFactory
// defined in the plugin. At most one instance of this plugin
// should exist at any point in the request chain
type Plugin struct {
	plugins.Core

	Factory Factory
}

// New returns a new instance of the session Plugin
// that uses the passed in factory to create session instances
func New(factory Factory) *Plugin {
	return &Plugin{
		Core:    plugins.Core{Id: "plugins.Session"},
		Factory: factory,
	}
}

// Handle lazily initiates a session instance per http request
// and stores the instance inside the Injections instance inside the verto Context
func (plugin *Plugin) Handle(c *verto.Context, next http.HandlerFunc) {
	plugin.Core.Handle(
		func(c *verto.Context, next http.HandlerFunc) {
			c.Injections().Lazy(SESSIONKEY,
				func(w http.ResponseWriter, r *http.Request, i verto.ReadOnlyInjections) interface{} {
					return plugin.Factory.Create(w, r)
				}, verto.REQUEST)
		}, c, next)
}

// Session is an interface for interacting with session
// data. Session implementations must be thread-safe
type Session interface {
	// Get retrieves the session value associated with
	// the passed in key or nil if no such value exists
	Get(key interface{}) interface{}

	// Set sets a key-value association for a session
	// instance. If an old association exists, it is
	// overwritten
	Set(key, value interface{})

	// Del deletes a key-value association from the session
	// instance
	Del(key interface{})

	// Clear clears a session instance of all data.
	Clear()

	// Flush writes session data to the backing store
	// for the session instance. Any errors encountered
	// writing session data are returned
	Flush() error
}

// CookieSession is an implementation of the Session
// interface using secure cookies as the backing store.
// CookieSession is thread safe
type CookieSession struct {
	data       map[interface{}]interface{}
	hashKey    []byte
	encryptKey []byte
	mutex      *sync.RWMutex
	w          http.ResponseWriter
	model      *http.Cookie
}

// Get retrieves the data associated with the key
// or nil if no such association exists
func (s *CookieSession) Get(key interface{}) interface{} {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return s.data[key]
}

// Set sets a key-value association for the session instance.
// If a previous association exists, it is overwritten
func (s *CookieSession) Set(key, value interface{}) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.data[key] = value
}

// Del deletes a key-value association from the sesion instance
func (s *CookieSession) Del(key interface{}) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	delete(s.data, key)
}

// Clear clears all data from the session instance.
// Calling clear and then flush will expire any cookies
// related to the session instance
func (s *CookieSession) Clear() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.data = make(map[interface{}]interface{})
}

// Flush writes any session data to the cookie backing
// store. Flush should only be called at the end of the
// request chain. If there is no data in the session instance,
// Flush will delete any associated cookies. Otherwise,
// the data will be marshalled and encoded into a secure cookie
// with the parameters set by the CookieSessionFactory that
// spawned the session instance
func (s *CookieSession) Flush() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// If no data, clear session cookie
	if len(s.data) == 0 {
		http.SetCookie(s.w, &http.Cookie{
			Name:    SESSIONKEY,
			Expires: time.Now().UTC(),
			MaxAge:  -1,
		})
		return nil
	}

	// attempt to marshal data map to json
	m, e := json.Marshal(s.data)
	if e != nil {
		return e
	}

	// attempt to secure cookie with HMAC and encryption,
	// then flush cookie to ResponseWriter and return
	s.model.Value = string(m)
	if secure, e := NewSecureCookie(s.model, s.hashKey, s.encryptKey); e != nil {
		return e
	} else {
		http.SetCookie(s.w, secure)
		return nil
	}
}

// Factory is an interface for creating Session instances
// from an http request
type Factory interface {
	Create(w http.ResponseWriter, r *http.Request) Session
}

// CookieSessionFactory is an implementation of SessionFactory
// that creates Session instances backed by secure cookies.
type CookieSessionFactory struct {
	// HashKey used to create an HMAC for the secure cookie
	// backing store. This field is required.
	HashKey []byte

	// EncryptKey is an optional key used to cryptographically
	// encrypt the contents of the secure cookie. If no
	// EncryptKey is provided, no encryption is done on the
	// secure cookie
	EncryptKey []byte

	// The below fields correspond to the fields within http.Cookie
	Path     string
	Domain   string
	Expires  time.Time
	MaxAge   int
	Secure   bool
	HttpOnly bool
}

// Create instantiates a CookieSession from the passed in http.Request
// and writes out to the passed in http.ResponseWriter. If the request
// contains an existing session cookie, the cookie will be decrypted and
// the contents stored in the generated session. If cookie decryption fails,
// the session data will be empty
func (factory *CookieSessionFactory) Create(w http.ResponseWriter, r *http.Request) Session {
	session := &CookieSession{
		data:       make(map[interface{}]interface{}),
		hashKey:    factory.HashKey,
		encryptKey: factory.EncryptKey,
		mutex:      &sync.RWMutex{},
		w:          w,

		model: &http.Cookie{
			Name:     SESSIONKEY,
			Path:     factory.Path,
			Domain:   factory.Domain,
			Expires:  factory.Expires,
			MaxAge:   factory.MaxAge,
			Secure:   factory.Secure,
			HttpOnly: factory.HttpOnly,
		},
	}

	// If a previous session exists and is valid,
	// unmarshal values into created session data
	if cookie, err := r.Cookie(SESSIONKEY); err == nil {
		if cookie, err := DecryptCookie(cookie, factory.HashKey, factory.EncryptKey); err == nil {
			json.Unmarshal([]byte(cookie.Value), session.data)
		}
	}

	return session
}

// NewSecureCookie returns a clone of the original cookie with the value
// encoded with a calculated MAC. If encryptKey is not nil, encryption will
// be performed on the value as well. hashKey must not be nil or ErrMissingKey
// will be returned. An is also returned in the case that encryption fails
func NewSecureCookie(cookie *http.Cookie, hashKey, encryptKey []byte) (*http.Cookie, error) {
	if hashKey == nil {
		return nil, ErrMissingKey
	}

	sc := clone(cookie)
	val := cookie.Value

	// Generate and append hmac of name + value to cookie
	sc.Value = val + sep + string(genHMAC(hashKey, sc.Name, val))

	if encryptKey != nil {
		// Init aes cipher and encrypt value with appended hmac
		block, err := aes.NewCipher(encryptKey)
		if err != nil {
			return nil, err
		}
		ciphertext := make([]byte, len(sc.Value)+aes.BlockSize)
		iv := ciphertext[:aes.BlockSize]
		if _, err := io.ReadFull(rand.Reader, iv); err != nil {
			return nil, err
		}
		cfb := cipher.NewCFBEncrypter(block, iv)
		cfb.XORKeyStream(ciphertext[aes.BlockSize:], []byte(sc.Value))

		// Set new cookie value as base64 encoded encrypted value + hmac
		sc.Value = base64.StdEncoding.EncodeToString(ciphertext)
	} else {
		sc.Value = base64.StdEncoding.EncodeToString([]byte(sc.Value))
	}
	return sc, nil
}

// DecryptCookie attempts to use hashKey and encryptKey to decrypt the value
// of the passed in cookie and return a read-only decrypted http.Cookie.
// The hashKey and encryptKey should match those used to encrypt the cookie
// originally. hashKey must not be nil or this function will return ErrMissingKey.
// An err is also returned if there was an issue performing decryption on the cookie
// or if the calculated MAC doesn't match the one stored in the cookie.
func DecryptCookie(cookie *http.Cookie, hashKey, encryptKey []byte) (*http.Cookie, error) {
	if hashKey == nil {
		return nil, ErrMissingKey
	}

	// Attempt to decrypt string using base64 encoding
	b, err := base64.StdEncoding.DecodeString(cookie.Value)
	if err != nil {
		return nil, err
	}

	var value string
	if encryptKey != nil {
		// Init aes cipher and decrypt bytes
		block, err := aes.NewCipher(encryptKey)
		if err != nil {
			return nil, err
		}
		if len(b) < aes.BlockSize {
			return nil, ErrCipherTooShort
		}
		iv := b[:aes.BlockSize]
		text := b[aes.BlockSize:]
		cfb := cipher.NewCFBDecrypter(block, iv)
		cfb.XORKeyStream(text, text)
		value = string(text)
	} else {
		value = string(b)
	}

	// Check HMAC and set actual value if successful,
	// return error otherwise
	if actual, pass := checkHMAC(hashKey, cookie.Name, value); !pass {
		return nil, ErrBadHMAC
	} else {
		value = actual
	}
	return &http.Cookie{Name: cookie.Name, Value: value}, nil
}

// separator string in order to
// separate hmac from rest of cookie value
var sep = ":"

// creates a for-write (Set-Cookie) clone
// of cookie
func clone(cookie *http.Cookie) *http.Cookie {
	return &http.Cookie{
		Name:    cookie.Name,
		Path:    cookie.Path,
		Domain:  cookie.Domain,
		Expires: cookie.Expires,
		MaxAge:  cookie.MaxAge,
		Secure:  cookie.Secure,
	}
}

// attempts to retrieve the mac from the value and compare
// against a freshly calculated mac using the passed in name
// and stripped value. Returns the stripped value and true
// if the mac matches or an empty string and false otherwise
func checkHMAC(key []byte, name, value string) (string, bool) {
	i := strings.LastIndex(value, sep)
	if i < 0 || i == len(value)-1 {
		return "", false
	}
	actual := value[:i]
	mac := value[i+1:]
	check := genHMAC(key, name, actual)
	if !hmac.Equal([]byte(mac), check) {
		return "", false
	}
	return actual, true
}

// genHMAC generates an HMAC from a cookie name and value using
// the given key
func genHMAC(key []byte, name, value string) []byte {
	var b bytes.Buffer
	b.WriteString(name)
	b.WriteString(value)
	mac := hmac.New(sha256.New, key)
	mac.Write(b.Bytes())
	return mac.Sum(nil)
}
