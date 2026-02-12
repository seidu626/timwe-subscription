package x

import (
	"google.golang.org/protobuf/types/known/timestamppb"
	"time"
)

func ConvertToTimestamp(tm time.Time) *timestamppb.Timestamp {
	//s := int64(tm.Second())     // from 'int'
	//n := int32(tm.Nanosecond()) // from 'int'
	return timestamppb.New(tm)
}
