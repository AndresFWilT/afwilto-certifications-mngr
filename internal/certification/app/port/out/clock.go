package out

import "time"

// Clock exists so domain tests are deterministic (fixed instant fakes).
type Clock interface {
	Now() time.Time
}
