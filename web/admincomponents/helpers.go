package admincomponents

import "time"

// TODO: get tz from request? And convert to that
func ConvertToLocalTZ(t time.Time) time.Time {
	tz, err := time.LoadLocation("Pacific/Auckland")
	if err != nil {
		return t
	}
	return t.In(tz)
}
