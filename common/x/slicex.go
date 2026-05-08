package x

// ConvertToSlicePtr converts a pointer to slices to  slice with pointer references
func ConvertToSlicePtr[S any](slices *[]S) []*S {
	records := make([]*S, len(*slices))
	for i, s := range *slices {
		records[i] = &s
	}
	return records
}

// ConvertToSlicePtrType converts a pointer to slices to  slice with pointer references
func ConvertToSlicePtrType[S any, O any](slices *[]S) []*O {
	records := make([]*O, len(*slices))
	for i, s := range *slices {
		records[i], _ = TypeConverter[O](&s)
	}
	return records
}

// ConvertToPtrSlice converts slice with pointer references to a pointer of slices
func ConvertToPtrSlice[S any](slices []*S) *[]S {
	records := make([]S, len(slices))
	for i, s := range slices {
		records[i] = *s
	}
	return &records
}

// ConvertToPtrSliceType converts slice with pointer references to a pointer of slices
func ConvertToPtrSliceType[S any, O any](slices []*S) *[]O {
	records := make([]O, len(slices))
	for i, s := range slices {
		val, _ := TypeConverter[O](&s)
		records[i] = *val
	}
	return &records
}
