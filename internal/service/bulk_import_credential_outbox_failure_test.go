package service

import (
	"errors"
	"net"
	"net/textproto"
	"testing"
)

func TestAccountSetupFailureClassification(t *testing.T) {
	cases := []struct {
		name string
		err error
		want string
	}{
		{name: "HTTP 429 throttling", err: &providerHTTPError{statusCode: 429}, want: "provider_unavailable"},
		{name: "HTTP 503 outage", err: &providerHTTPError{statusCode: 503}, want: "provider_unavailable"},
		{name: "HTTP 408 timeout", err: &providerHTTPError{statusCode: 408}, want: "provider_unavailable"},
		{name: "HTTP 400 rejected recipient", err: &providerHTTPError{statusCode: 400}, want: "provider_rejected"},
		{name: "SMTP 450 temporary failure", err: &textproto.Error{Code: 450, Msg: "temporary failure"}, want: "provider_unavailable"},
		{name: "SMTP 550 permanent rejection", err: &textproto.Error{Code: 550, Msg: "mailbox unavailable"}, want: "provider_rejected"},
		{name: "network timeout", err: &net.DNSError{IsTimeout: true}, want: "provider_timeout"},
		{name: "network unavailable", err: &net.OpError{Op: "dial", Net: "tcp", Err: errors.New("connection refused")}, want: "provider_unavailable"},
		{name: "unclassified error", err: errors.New("invalid recipient"), want: "provider_rejected"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := accountSetupFailureCode(tc.err); got != tc.want {
				t.Fatalf("accountSetupFailureCode() = %q, want %q", got, tc.want)
			}
		})
	}
}
