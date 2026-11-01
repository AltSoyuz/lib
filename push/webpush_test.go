package push

import (
	"crypto/ecdh"
	"crypto/rand"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// The example of RFC 8291, section 5 and appendix A.
func TestSealMatchesRFC8291(t *testing.T) {
	raw, err := b64.DecodeString("yfWPiYE-n46HLnH0KqZOF1fJJU3MYrct3AELtAQ-oRw")
	if err != nil {
		t.Fatal(err)
	}
	as, err := ecdh.P256().NewPrivateKey(raw)
	if err != nil {
		t.Fatal(err)
	}
	salt, err := b64.DecodeString("DGv6ra1nlYgDCS1FRnbzlw")
	if err != nil {
		t.Fatal(err)
	}
	got, err := seal([]byte("When I grow up, I want to be a watermelon"),
		"BCVxsr7N_eNgVRqvHtD0zTZsEc6-VV-JvLexhqUzORcxaOzi6-AYWXvTBHm4bjyPjs7Vd8pZGH6SRpkNtoIAiw4", "BTBZMqHH6r4Tts7J_aSIgg", salt, as)
	if err != nil {
		t.Fatal(err)
	}
	want := "DGv6ra1nlYgDCS1FRnbzlwAAEABBBP4z9KsN6nGRTbVYI_c7VJSPQTBtkgcy27ml" +
		"mlMoZIIgDll6e3vCYLocInmYWAmS6TlzAC8wEqKK6PBru3jl7A_yl95bQpu6cVPT" +
		"pK4Mqgkf1CXztLVBSt2Ks3oZwbuwXPXLWyouBWLVWGNWQexSgSxsj_Qulcy4a-fN"
	if b64.EncodeToString(got) != want {
		t.Fatalf("body\n%s\nwant\n%s", b64.EncodeToString(got), want)
	}
}

func TestEncryptRefusesWhatAPushServiceWouldNot(t *testing.T) {
	ua, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	p256dh := b64.EncodeToString(ua.PublicKey().Bytes())
	if _, err := encrypt([]byte("hi"), p256dh, "BTBZMqHH6r4Tts7J_aSIgg"); err != nil {
		t.Fatalf("a valid message: %v", err)
	}
	for name, c := range map[string]struct {
		payload      []byte
		p256dh, auth string
	}{
		"payload over one record": {make([]byte, recordSize), p256dh, "BTBZMqHH6r4Tts7J_aSIgg"},
		"short auth secret":       {[]byte("hi"), p256dh, "BTBZMqHH6r4T"},
		"not a point":             {[]byte("hi"), "BCVxsr7N", "BTBZMqHH6r4Tts7J_aSIgg"},
	} {
		if _, err := encrypt(c.payload, c.p256dh, c.auth); err == nil {
			t.Errorf("%s: encrypted", name)
		}
	}
}

func TestAuthorizationIsAVapidJWTForTheEndpointOrigin(t *testing.T) {
	keys, private, err := NewKeys()
	if err != nil {
		t.Fatal(err)
	}
	again, err := ParseKeys(private)
	if err != nil || again.Public != keys.Public {
		t.Fatalf("ParseKeys(NewKeys) = %v, %v", again.Public, err)
	}
	now := time.Unix(1_800_000_000, 0)
	h, err := keys.authorization("https://web.push.apple.com/QGuQyavXutnMH...", "mailto:admin@example.org", now)
	if err != nil {
		t.Fatal(err)
	}
	jwt, key, ok := strings.Cut(strings.TrimPrefix(h, "vapid t="), ", k=")
	if !ok || key != keys.Public || !verifyJWT(keys.Public, jwt) {
		t.Fatalf("header %q does not verify", h)
	}
	other, _, _ := NewKeys()
	if verifyJWT(other.Public, jwt) {
		t.Fatal("the JWT verifies with another key")
	}
	payload, err := b64.DecodeString(strings.Split(jwt, ".")[1])
	if err != nil {
		t.Fatal(err)
	}
	var claims struct {
		Aud string `json:"aud"`
		Exp int64  `json:"exp"`
		Sub string `json:"sub"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		t.Fatal(err)
	}
	if claims.Aud != "https://web.push.apple.com" || claims.Exp != now.Add(12*time.Hour).Unix() || claims.Sub != "mailto:admin@example.org" {
		t.Fatalf("claims = %+v", claims)
	}
}
