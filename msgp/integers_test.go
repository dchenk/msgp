package msgp

import (
	"testing"
)

func TestPutGetUnix(t *testing.T) {

	cases := []struct {
		seconds     int64
		nanoseconds int32
	}{
		{1523767262, 20},
		{10167812, 115},
		{15667812, 0},
	}

	buf := make([]byte, 12)

	for i, tc := range cases {
		putUnix(buf, tc.seconds, tc.nanoseconds)
		sec, nsec := getUnix(buf)
		if sec != tc.seconds {
			t.Errorf("(index %d) got bad seconds: %d", i, sec)
		}
		if nsec != tc.nanoseconds {
			t.Errorf("(index %d) got bad nanoseconds: %d", i, nsec)
		}
	}

}
