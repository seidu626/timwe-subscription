-- Records why a subscription_message_state was stopped so that console-driven
-- series reactivation can resume exactly the states stopped by deactivation
-- ('series_inactive') without resurrecting content-exhausted ('no_content')
-- or otherwise terminal states.
ALTER TABLE subscription_message_state ADD COLUMN IF NOT EXISTS stop_reason TEXT;
