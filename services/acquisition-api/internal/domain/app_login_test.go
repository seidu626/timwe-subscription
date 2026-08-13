package domain

import "testing"

func TestMapTransactionStatusToApp(t *testing.T) {
	cases := []struct {
		status TransactionStatus
		want   string
	}{
		{StatusSubscribed, "ACTIVE"},
		{StatusCharged, "ACTIVE"},
		{StatusPending, "PENDING"},
		{StatusActionRequired, "PENDING"},
		{StatusConfirmRequired, "PENDING"},
		{StatusCancelled, "CANCELLED"},
		{StatusFailed, "FAILED"},
		{TransactionStatus("SOMETHING_ELSE"), "FAILED"},
	}
	for _, c := range cases {
		if got := MapTransactionStatusToApp(c.status); got != c.want {
			t.Errorf("MapTransactionStatusToApp(%q) = %q, want %q", c.status, got, c.want)
		}
	}
}
