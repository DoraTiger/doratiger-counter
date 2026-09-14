package config

import "testing"

func TestIsOriginAllowedMatchesNormalizedHostname(t *testing.T) {
	t.Parallel()

	cfg := &CounterConfig{AllowedOrigins: []string{"example.com", "https://www.example.com"}}
	tests := []struct {
		name   string
		origin string
		want   bool
	}{
		{name: "exact host", origin: "https://example.com", want: true},
		{name: "configured host with port", origin: "https://example.com:8443", want: true},
		{name: "configured URL", origin: "https://www.example.com/post/", want: true},
		{name: "case insensitive", origin: "HTTPS://EXAMPLE.COM", want: true},
		{name: "unconfigured subdomain", origin: "https://blog.example.com", want: false},
		{name: "malicious suffix", origin: "https://example.com.evil.test", want: false},
		{name: "userinfo trick", origin: "https://example.com@evil.test", want: false},
		{name: "malformed", origin: "://example.com", want: false},
		{name: "missing", origin: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := cfg.IsOriginAllowed(tt.origin); got != tt.want {
				t.Fatalf("IsOriginAllowed(%q) = %v, want %v", tt.origin, got, tt.want)
			}
		})
	}
}

func TestIsOriginAllowedWithoutAllowlist(t *testing.T) {
	t.Parallel()

	cfg := &CounterConfig{}
	if !cfg.IsOriginAllowed("") {
		t.Fatal("empty allowlist should allow requests without an Origin header")
	}
}

func TestResolveSiteKeyMapsConfiguredHostsWithoutDefaultFallback(t *testing.T) {
	t.Parallel()

	cfg := &CounterConfig{
		SiteKey: "dtc_site",
		Sites: map[string]string{
			"www.superheaoz.top": "dtc_site",
			"www.doratiger.top":  "doratiger_site",
		},
	}
	tests := []struct {
		name   string
		source string
		want   string
		ok     bool
	}{
		{name: "superheaoz", source: "https://www.superheaoz.top", want: "dtc_site", ok: true},
		{name: "doratiger", source: "https://www.doratiger.top", want: "doratiger_site", ok: true},
		{name: "unconfigured host", source: "https://www.evil.test", ok: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := cfg.ResolveSiteKey(tt.source)
			if ok != tt.ok || got != tt.want {
				t.Fatalf("ResolveSiteKey(%q) = (%q, %v), want (%q, %v)", tt.source, got, ok, tt.want, tt.ok)
			}
		})
	}
}
