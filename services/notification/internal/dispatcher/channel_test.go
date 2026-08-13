package dispatcher

import "testing"

func TestDecideChannel_PushWhenPreferredDeviceAndCredsPresent(t *testing.T) {
	got := decideChannel(channelDecisionInput{
		preferredChannel: "PUSH",
		hasDevice:        true,
		pushConfigured:   true,
	})
	if got != channelPush {
		t.Errorf("got %q, want PUSH", got)
	}
}

func TestDecideChannel_SMSWhenPushNotConfigured(t *testing.T) {
	got := decideChannel(channelDecisionInput{
		preferredChannel: "PUSH",
		hasDevice:        true,
		pushConfigured:   false,
	})
	if got != channelSMS {
		t.Errorf("got %q, want SMS", got)
	}
}

func TestDecideChannel_SMSWhenNoDevice(t *testing.T) {
	got := decideChannel(channelDecisionInput{
		preferredChannel: "PUSH",
		hasDevice:        false,
		pushConfigured:   true,
	})
	if got != channelSMS {
		t.Errorf("got %q, want SMS", got)
	}
}

func TestDecideChannel_SMSWhenPreferenceUnset(t *testing.T) {
	got := decideChannel(channelDecisionInput{
		preferredChannel: "",
		hasDevice:        true,
		pushConfigured:   true,
	})
	if got != channelSMS {
		t.Errorf("got %q, want SMS", got)
	}
}

func TestDecideChannel_SMSWhenPreferenceIsBoth(t *testing.T) {
	got := decideChannel(channelDecisionInput{
		preferredChannel: "BOTH",
		hasDevice:        true,
		pushConfigured:   true,
	})
	if got != channelSMS {
		t.Errorf("got %q, want SMS (literal PUSH-only reading of the contract)", got)
	}
}
