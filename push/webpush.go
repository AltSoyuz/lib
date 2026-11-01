package push

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/url"
	"strings"
	"time"
)

// recordSize is the one aes128gcm record a push message fits in: push services take 4096 octets at most.
const recordSize = 4096

var b64 = base64.RawURLEncoding

// Keys is the VAPID key pair (RFC 8292) the push services know this server by.
type Keys struct {
	private *ecdsa.PrivateKey
	// Public is the uncompressed P-256 point, base64url: the applicationServerKey the phones subscribe with.
	Public string
}

// NewKeys makes a fresh VAPID key pair, and the private key as the -push.vapidPrivateKey flag holds it.
func NewKeys() (Keys, string, error) {
	sk, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return Keys{}, "", err
	}
	raw, err := sk.Bytes()
	if err != nil {
		return Keys{}, "", err
	}
	text := b64.EncodeToString(raw)
	keys, err := ParseKeys(text)
	return keys, text, err
}

// ParseKeys reads a VAPID private key: the 32 octet P-256 scalar, base64url.
func ParseKeys(private string) (Keys, error) {
	raw, err := b64.DecodeString(private)
	if err != nil {
		return Keys{}, fmt.Errorf("VAPID private key is not base64url: %w", err)
	}
	sk, err := ecdsa.ParseRawPrivateKey(elliptic.P256(), raw)
	if err != nil {
		return Keys{}, fmt.Errorf("VAPID private key is not a P-256 scalar: %w", err)
	}
	pub, err := sk.PublicKey.Bytes()
	if err != nil {
		return Keys{}, err
	}
	return Keys{private: sk, Public: b64.EncodeToString(pub)}, nil
}

// authorization is the VAPID header of a request to endpoint: a JWT signed ES256 for its origin, and the public key.
func (k Keys) authorization(endpoint, subject string, now time.Time) (string, error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return "", err
	}
	claims, err := json.Marshal(map[string]any{"aud": u.Scheme + "://" + u.Host, "exp": now.Add(12 * time.Hour).Unix(), "sub": subject})
	if err != nil {
		return "", err
	}
	signing := b64.EncodeToString([]byte(`{"typ":"JWT","alg":"ES256"}`)) + "." + b64.EncodeToString(claims)
	digest := sha256.Sum256([]byte(signing))
	r, s, err := ecdsa.Sign(rand.Reader, k.private, digest[:])
	if err != nil {
		return "", err
	}
	sig := make([]byte, 64)
	r.FillBytes(sig[:32])
	s.FillBytes(sig[32:])
	return "vapid t=" + signing + "." + b64.EncodeToString(sig) + ", k=" + k.Public, nil
}

// verifyJWT checks a JWT signed by the public key, the way a push service reads the VAPID header.
func verifyJWT(public, jwt string) bool {
	raw, err := b64.DecodeString(public)
	if err != nil {
		return false
	}
	pk, err := ecdsa.ParseUncompressedPublicKey(elliptic.P256(), raw)
	if err != nil {
		return false
	}
	dot := strings.LastIndex(jwt, ".")
	sig, err := b64.DecodeString(jwt[dot+1:])
	if dot < 0 || err != nil || len(sig) != 64 {
		return false
	}
	digest := sha256.Sum256([]byte(jwt[:dot]))
	return ecdsa.Verify(pk, digest[:], new(big.Int).SetBytes(sig[:32]), new(big.Int).SetBytes(sig[32:]))
}

// encrypt makes the aes128gcm body of RFC 8291 for a browser's keys, with a fresh salt and sender key.
func encrypt(plaintext []byte, p256dh, authSecret string) ([]byte, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return nil, err
	}
	as, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	return seal(plaintext, p256dh, authSecret, salt, as)
}

func seal(plaintext []byte, p256dh, authSecret string, salt []byte, as *ecdh.PrivateKey) ([]byte, error) {
	uaRaw, err := b64.DecodeString(p256dh)
	if err != nil {
		return nil, fmt.Errorf("p256dh is not base64url: %w", err)
	}
	ua, err := ecdh.P256().NewPublicKey(uaRaw)
	if err != nil {
		return nil, fmt.Errorf("p256dh is not a P-256 point: %w", err)
	}
	secret, err := b64.DecodeString(authSecret)
	if err != nil || len(secret) != 16 {
		return nil, errors.New("auth is not a 16 octet base64url secret")
	}
	if len(plaintext)+1+16 > recordSize {
		return nil, fmt.Errorf("push payload of %d octets does not fit one record", len(plaintext))
	}
	shared, err := as.ECDH(ua)
	if err != nil {
		return nil, err
	}
	asPublic := as.PublicKey().Bytes()

	keyInfo := append(append([]byte("WebPush: info\x00"), uaRaw...), asPublic...)
	ikm := expand(extract(secret, shared), keyInfo, 32)
	prk := extract(salt, ikm)
	cek := expand(prk, []byte("Content-Encoding: aes128gcm\x00"), 16)
	nonce := expand(prk, []byte("Content-Encoding: nonce\x00"), 12)

	block, err := aes.NewCipher(cek)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	header := make([]byte, 0, 16+4+1+len(asPublic))
	header = append(header, salt...)
	header = binary.BigEndian.AppendUint32(header, recordSize)
	header = append(header, byte(len(asPublic)))
	header = append(header, asPublic...)
	// 0x02 closes the last (and only) record, without padding.
	return gcm.Seal(header, nonce, append(append([]byte{}, plaintext...), 0x02), nil), nil
}

// extract and expand are HKDF (RFC 5869) with SHA-256, expand limited to the one block RFC 8291 needs.
func extract(salt, ikm []byte) []byte {
	m := hmac.New(sha256.New, salt)
	m.Write(ikm)
	return m.Sum(nil)
}

func expand(prk, info []byte, n int) []byte {
	m := hmac.New(sha256.New, prk)
	m.Write(info)
	m.Write([]byte{1})
	return m.Sum(nil)[:n]
}
