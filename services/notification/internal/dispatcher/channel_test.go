package dispatcher

import "testing"

func TestDecideChannel(t *testing.T) {
	cases := []struct {
		name             string
		deliveryChannel  string
		preferredChannel string
		hasDevice        bool
		pushConfigured   bool
		want             string
	}{
		{name: "user pref push with device and creds", deliveryChannel: channelUserPref, preferredChannel: "PUSH", hasDevice: true, pushConfigured: true, want: channelPush},
		{name: "user pref push without creds falls back", deliveryChannel: channelUserPref, preferredChannel: "PUSH", hasDevice: true, pushConfigured: false, want: channelSMS},
		{name: "user pref push without device falls back", deliveryChannel: channelUserPref, preferredChannel: "PUSH", hasDevice: false, pushConfigured: true, want: channelSMS},
		{name: "user pref unset falls back", deliveryChannel: channelUserPref, preferredChannel: "", hasDevice: true, pushConfigured: true, want: channelSMS},
		{name: "user pref both keeps existing sms behavior", deliveryChannel: channelUserPref, preferredChannel: "BOTH", hasDevice: true, pushConfigured: true, want: channelSMS},
		{name: "blank delivery channel keeps existing behavior", preferredChannel: "PUSH", hasDevice: true, pushConfigured: true, want: channelPush},
		{name: "sms override forces sms", deliveryChannel: channelSMS, preferredChannel: "PUSH", hasDevice: true, pushConfigured: true, want: channelSMS},
		{name: "push override uses push when possible", deliveryChannel: channelPush, preferredChannel: "SMS", hasDevice: true, pushConfigured: true, want: channelPush},
		{name: "push override falls back without device", deliveryChannel: channelPush, preferredChannel: "SMS", hasDevice: false, pushConfigured: true, want: channelSMS},
		{name: "push override falls back without creds", deliveryChannel: channelPush, preferredChannel: "SMS", hasDevice: true, pushConfigured: false, want: channelSMS},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := decideChannel(channelDecisionInput{
				preferredChannel: tc.preferredChannel,
				hasDevice:        tc.hasDevice,
				pushConfigured:   tc.pushConfigured,
				deliveryChannel:  tc.deliveryChannel,
			})
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}
