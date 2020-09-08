package display

import (
	"testing"
	"time"
)

func TestTimeAgo(t *testing.T) {
	t0 := time.Now()
	monthAgo := t0.Add(-(30 * 24) * time.Hour)

	tests := []struct {
		ts   time.Time
		want string
	}{
		{t0, "a minute ago"},
		{t0.Add(-2 * time.Minute), "2 minutes ago"},
		{t0.Add(-119 * time.Minute), "an hour ago"},
		{t0.Add(-3 * time.Hour), "3 hours ago"},
		{t0.Add(-23 * time.Hour), "23 hours ago"},
		{t0.Add(-24 * time.Hour), "a day ago"},
		{t0.Add(-48 * time.Hour), "2 days ago"},
		{t0.Add(-(29 * 24) * time.Hour), "29 days ago"},
		{monthAgo, monthAgo.Format("Jan 02 2006")},
	}

	for _, test := range tests {
		t.Run(test.want, func(t *testing.T) {
			got := timeAgo(test.ts)

			if test.want != got {
				t.Fatalf("wanted %q, got %q", test.want, got)
			}
		})
	}
}
