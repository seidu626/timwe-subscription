package handler

import (
	"errors"
	"testing"

	"github.com/valyala/fasthttp"
)

func TestMapCreateTransactionStatus(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{name: "throttled", err: errors.New("request throttled"), want: fasthttp.StatusTooManyRequests},
		{name: "campaign missing", err: errors.New("campaign not found"), want: fasthttp.StatusNotFound},
		{name: "default bad request", err: errors.New("invalid msisdn format"), want: fasthttp.StatusBadRequest},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := mapCreateTransactionStatus(tc.err)
			if got != tc.want {
				t.Fatalf("expected %d, got %d", tc.want, got)
			}
		})
	}
}

func TestMapConfirmTransactionStatus(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{name: "not found", err: errors.New("transaction not found"), want: fasthttp.StatusNotFound},
		{name: "wrong state", err: errors.New("transaction is not in confirm_required status"), want: fasthttp.StatusConflict},
		{name: "default bad request", err: errors.New("invalid auth code"), want: fasthttp.StatusBadRequest},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := mapConfirmTransactionStatus(tc.err)
			if got != tc.want {
				t.Fatalf("expected %d, got %d", tc.want, got)
			}
		})
	}
}
