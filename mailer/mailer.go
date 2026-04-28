package mailer

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/smtp"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/AltSoyuz/lib/logger"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	sesv2 "github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/aws/aws-sdk-go-v2/service/sesv2/types"
)

type mode uint8

const (
	modeBlackhole mode = iota
	modeSMTP
	modeSES
	modeResend
)

var (
	curMode      = modeBlackhole
	blackholeErr atomic.Value
	smtpCfg      smtpSender
	sesCfg       ses
	resendCfg    resendSender
)

type initConfig struct {
	mode             mode
	from             string
	smtpAddr         string
	smtpHost         string
	smtpUsername     string
	smtpPassword     string
	region           string
	resendAPIKey     string
	resendAPIBaseURL string
}

func Init(from, smtpAddr, smtpUsername, smtpPassword, region, resendAPIKey, resendAPIBaseURL string) {
	cfg, err := buildConfig(from, smtpAddr, smtpUsername, smtpPassword, region, resendAPIKey, resendAPIBaseURL)
	if err != nil {
		logger.Fatal("mailer init failed", "err", err)
	}

	switch cfg.mode {
	case modeBlackhole:
		curMode = modeBlackhole
		logger.WarnSkipframes(1, "mailer not configured using blackhole")
	case modeSMTP:
		smtpCfg = smtpSender{
			from:       cfg.from,
			smtpAddr:   cfg.smtpAddr,
			host:       cfg.smtpHost,
			username:   cfg.smtpUsername,
			password:   cfg.smtpPassword,
			requireTLS: cfg.smtpUsername != "",
		}
		curMode = modeSMTP
		logger.Info("mailer smtp configured", "addr", "smtp://"+cfg.smtpAddr)
	case modeSES:
		sesCfg = ses{from: cfg.from, region: cfg.region, addrStr: "ses://" + cfg.region}
		if err := sesCfg.init(context.Background()); err != nil {
			logger.Fatal("mailer ses init failed", "err", err)
		}
		curMode = modeSES
		logger.Info("mailer ses configured", "addr", sesCfg.addrStr)
	case modeResend:
		resendCfg = resendSender{
			from:    cfg.from,
			apiKey:  cfg.resendAPIKey,
			baseURL: cfg.resendAPIBaseURL,
			client:  &http.Client{Timeout: 7 * time.Second},
		}
		curMode = modeResend
		logger.Info("mailer resend configured", "addr", resendCfg.addr())
	default:
		logger.Fatal("mailer init failed", "err", fmt.Errorf("unknown mailer mode %d", cfg.mode))
	}
}

func buildConfig(from, smtpAddr, smtpUsername, smtpPassword, region, resendAPIKey, resendAPIBaseURL string) (initConfig, error) {
	resendAPIKey = strings.TrimSpace(resendAPIKey)
	resendAPIBaseURL = strings.TrimSpace(resendAPIBaseURL)
	if smtpUsername != "" && smtpPassword == "" {
		return initConfig{}, fmt.Errorf("mailer smtp password is required when smtp username is set")
	}
	if smtpPassword != "" && smtpUsername == "" {
		return initConfig{}, fmt.Errorf("mailer smtp username is required when smtp password is set")
	}
	if smtpAddr == "" && (smtpUsername != "" || smtpPassword != "") {
		return initConfig{}, fmt.Errorf("mailer smtp address is required when smtp credentials are set")
	}
	if resendAPIBaseURL != "" && resendAPIKey == "" {
		return initConfig{}, fmt.Errorf("mailer resend api key is required when resend api base url is set")
	}

	providers := 0
	if smtpAddr != "" {
		providers++
	}
	if region != "" {
		providers++
	}
	if resendAPIKey != "" {
		providers++
	}
	if providers == 0 {
		if from != "" {
			return initConfig{}, fmt.Errorf("mailer mode is missing for from address %q", from)
		}
		return initConfig{mode: modeBlackhole}, nil
	}
	if providers > 1 {
		return initConfig{}, fmt.Errorf("multiple mailer modes configured; set only one of mail.smtpAddr, mail.sesRegion, or mail.resendApiKey")
	}
	if from == "" {
		return initConfig{}, fmt.Errorf("mailer from address is required")
	}
	if smtpAddr != "" {
		host, _, err := net.SplitHostPort(smtpAddr)
		if err != nil {
			return initConfig{}, fmt.Errorf("mailer smtp init failed: %w", err)
		}
		if host == "" {
			return initConfig{}, fmt.Errorf("mailer smtp init failed: %w", fmt.Errorf("smtp host is empty"))
		}
		return initConfig{
			mode:         modeSMTP,
			from:         from,
			smtpAddr:     smtpAddr,
			smtpHost:     host,
			smtpUsername: smtpUsername,
			smtpPassword: smtpPassword,
		}, nil
	}
	if resendAPIKey != "" {
		baseURL := resendAPIBaseURL
		if baseURL == "" {
			baseURL = "https://api.resend.com"
		}
		u, err := url.Parse(baseURL)
		if err != nil || u.Scheme == "" || u.Host == "" {
			return initConfig{}, fmt.Errorf("mailer resend api base url is invalid: %q", baseURL)
		}
		if u.Scheme != "https" && u.Scheme != "http" {
			return initConfig{}, fmt.Errorf("mailer resend api base url must be http or https")
		}
		return initConfig{
			mode:             modeResend,
			from:             from,
			resendAPIKey:     resendAPIKey,
			resendAPIBaseURL: strings.TrimRight(baseURL, "/"),
		}, nil
	}
	return initConfig{mode: modeSES, from: from, region: region}, nil
}

func UseBlackHole() { curMode = modeBlackhole }

func Send(ctx context.Context, to, subject, text, html string) error {
	switch curMode {
	case modeSMTP:
		return smtpCfg.send(ctx, to, subject, text, html)
	case modeSES:
		return sesCfg.send(ctx, to, subject, text, html)
	case modeResend:
		return resendCfg.send(ctx, to, subject, text, html)
	default:
		blackholeErr.Store("blackhole send to " + to)
		return nil
	}
}

func Addr() string {
	switch curMode {
	case modeSMTP:
		return "smtp://" + smtpCfg.smtpAddr
	case modeSES:
		return sesCfg.addrStr
	case modeResend:
		return resendCfg.addr()
	default:
		return "blackhole"
	}
}

func LastError() string {
	switch curMode {
	case modeSMTP:
		v, _ := smtpCfg.last.Load().(string)
		return v
	case modeSES:
		v, _ := sesCfg.last.Load().(string)
		return v
	case modeResend:
		v, _ := resendCfg.last.Load().(string)
		return v
	default:
		v, _ := blackholeErr.Load().(string)
		return v
	}
}

func Close() {}

type smtpSender struct {
	from       string
	smtpAddr   string
	host       string
	username   string
	password   string
	requireTLS bool
	last       atomic.Value
}

func (s *smtpSender) send(ctx context.Context, to, subject, text, html string) error {
	if text == "" && html == "" {
		text = " "
	}
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", s.smtpAddr)
	if err != nil {
		s.last.Store(err.Error())
		return err
	}
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}
	client, err := smtp.NewClient(conn, s.host)
	if err != nil {
		_ = conn.Close()
		s.last.Store(err.Error())
		return err
	}
	defer func() {
		_ = client.Close()
	}()

	if ok, _ := client.Extension("STARTTLS"); ok {
		cfg := &tls.Config{MinVersion: tls.VersionTLS12, ServerName: s.host}
		if err := client.StartTLS(cfg); err != nil {
			s.last.Store(err.Error())
			return err
		}
	} else if s.requireTLS {
		err := fmt.Errorf("smtp server %q does not advertise STARTTLS", s.smtpAddr)
		s.last.Store(err.Error())
		return err
	}

	if s.username != "" {
		if ok, _ := client.Extension("AUTH"); !ok {
			err := fmt.Errorf("smtp server %q does not advertise AUTH", s.smtpAddr)
			s.last.Store(err.Error())
			return err
		}
		auth := smtp.PlainAuth("", s.username, s.password, s.host)
		if err := client.Auth(auth); err != nil {
			s.last.Store(err.Error())
			return err
		}
	}

	if err := client.Mail(s.from); err != nil {
		s.last.Store(err.Error())
		return err
	}
	if err := client.Rcpt(to); err != nil {
		s.last.Store(err.Error())
		return err
	}
	wc, err := client.Data()
	if err != nil {
		s.last.Store(err.Error())
		return err
	}
	msg := strings.Builder{}
	msg.WriteString("From: ")
	msg.WriteString(s.from)
	msg.WriteString("\r\n")
	msg.WriteString("To: ")
	msg.WriteString(to)
	msg.WriteString("\r\n")
	msg.WriteString("Subject: ")
	msg.WriteString(subject)
	msg.WriteString("\r\n")
	msg.WriteString("MIME-Version: 1.0\r\n")
	if html != "" {
		boundary := "mime-boundary"
		msg.WriteString("Content-Type: multipart/alternative; boundary=")
		msg.WriteString(boundary)
		msg.WriteString("\r\n\r\n")
		msg.WriteString("--")
		msg.WriteString(boundary)
		msg.WriteString("\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n")
		msg.WriteString(text)
		msg.WriteString("\r\n--")
		msg.WriteString(boundary)
		msg.WriteString("\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n")
		msg.WriteString(html)
		msg.WriteString("\r\n--")
		msg.WriteString(boundary)
		msg.WriteString("--\r\n")
	} else {
		msg.WriteString("Content-Type: text/plain; charset=UTF-8\r\n\r\n")
		msg.WriteString(text)
	}
	if _, err := wc.Write([]byte(msg.String())); err != nil {
		_ = wc.Close()
		s.last.Store(err.Error())
		return err
	}
	if err := wc.Close(); err != nil {
		s.last.Store(err.Error())
		return err
	}
	if err := client.Quit(); err != nil {
		s.last.Store(err.Error())
		return err
	}
	return nil
}

type ses struct {
	from    string
	region  string
	cl      *sesv2.Client
	once    sync.Once
	initErr error
	addrStr string
	last    atomic.Value
}

func (s *ses) init(ctx context.Context) error {
	s.once.Do(func() {
		logger.Info("initializing aws ses client", "region", s.region)
		cfg, err := config.LoadDefaultConfig(ctx,
			config.WithRegion(s.region),
			config.WithRetryMaxAttempts(1),
		)
		if err != nil {
			s.initErr = err
			logger.Error("aws config load failed", "err", err)
			return
		}
		s.cl = sesv2.NewFromConfig(cfg)
		logger.Info("aws ses client created, starting warmup")

		c2, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		if err := s.warmup(c2); err != nil {
			s.initErr = err
			logger.Error("ses warmup failed", "err", err)
			return
		}
		logger.Info("ses warmup completed successfully")
	})
	return s.initErr
}

func (s *ses) warmup(ctx context.Context) error {
	out, err := s.cl.GetAccount(ctx, &sesv2.GetAccountInput{})
	if err != nil {
		return fmt.Errorf("SES GetAccount failed: %w", err)
	}

	if out.SendQuota != nil {
		var max24, rate float64
		max24 = out.SendQuota.Max24HourSend
		rate = out.SendQuota.MaxSendRate
		s.addrStr = fmt.Sprintf("ses://%s (max24=%.0f rate=%.1f)", s.region, max24, rate)
	}

	ident := s.from
	if i := strings.LastIndexByte(ident, '@'); i > 0 {
		ident = ident[i+1:]
	}
	_, err = s.cl.GetEmailIdentity(ctx, &sesv2.GetEmailIdentityInput{
		EmailIdentity: aws.String(ident),
	})
	if err != nil {
		return fmt.Errorf("SES GetEmailIdentity failed for %q: %w", ident, err)
	}
	return nil
}

func (s *ses) send(ctx context.Context, to, subject, text, html string) error {
	if err := s.init(ctx); err != nil {
		s.last.Store(err.Error())
		return err
	}
	if text == "" && html == "" {
		text = " " // SES refuses empty body
	}
	in := &sesv2.SendEmailInput{
		FromEmailAddress: aws.String(s.from),
		Destination:      &types.Destination{ToAddresses: []string{to}},
		Content: &types.EmailContent{
			Simple: &types.Message{
				Subject: &types.Content{Data: aws.String(subject)},
				Body: &types.Body{
					Text: strContent(text),
					Html: strContent(html),
				},
			},
		},
	}

	var last error
	for i := range 3 {
		c2, cancel := context.WithTimeout(ctx, 5*time.Second)
		_, err := s.cl.SendEmail(c2, in)
		cancel()
		if err == nil {
			return nil
		}
		last = err
		s.last.Store(err.Error())
		select {
		case <-ctx.Done():
			return last
		case <-time.After(time.Duration(i+1) * 200 * time.Millisecond):
		}
	}
	return last
}

func strContent(s string) *types.Content {
	if s == "" {
		return nil
	}
	return &types.Content{Data: aws.String(s)}
}

type resendSender struct {
	from    string
	apiKey  string
	baseURL string
	client  *http.Client
	last    atomic.Value
}

func (s *resendSender) addr() string {
	return "resend+" + s.baseURL
}

func (s *resendSender) send(ctx context.Context, to, subject, text, html string) error {
	if text == "" && html == "" {
		text = " "
	}

	payload := map[string]any{
		"from":    s.from,
		"to":      []string{to},
		"subject": subject,
	}
	if text != "" {
		payload["text"] = text
	}
	if html != "" {
		payload["html"] = html
	}
	body, err := json.Marshal(payload)
	if err != nil {
		s.last.Store(err.Error())
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+"/emails", bytes.NewReader(body))
	if err != nil {
		s.last.Store(err.Error())
		return err
	}
	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	req.Header.Set("Content-Type", "application/json")

	res, err := s.client.Do(req)
	if err != nil {
		s.last.Store(err.Error())
		return err
	}
	defer func() {
		_ = res.Body.Close()
	}()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
		err := fmt.Errorf("resend send failed: status=%d body=%q", res.StatusCode, string(b))
		s.last.Store(err.Error())
		return err
	}

	return nil
}

// BuildVerifyURL builds a verification link.
func BuildVerifyURL(base, path, token string) (string, error) {
	u, err := url.Parse(base)
	if err != nil {
		return "", err
	}
	u.Path = path
	q := u.Query()
	q.Set("token", token)
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func IsBlackhole() bool {
	return curMode == modeBlackhole
}
