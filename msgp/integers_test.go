package msgp

import (
	"testing"
)

func TestPutGetInt64(t *testing.T) {

	nums := []int64{0, -4, 235, -56475462, 23423423525, -9223372036854775808, 9223372036854775807}

	scratch := make([]byte, 9)

	for _, i := range nums {

		putMint64(scratch, i)
		got := getMint64(scratch)
		if got != i {
			t.Errorf("put in %d but got out %d", i, got)
		}

	}

}

func TestPutGetInt32(t *testing.T) {

	nums := []int32{0, -4, 235, -6959, 234525, -2147483648, 2147483647}

	scratch := make([]byte, 5)

	for _, i := range nums {

		putMint32(scratch, i)
		got := getMint32(scratch)
		if got != i {
			t.Errorf("put in %d but got out %d", i, got)
		}

	}

}

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
