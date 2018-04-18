package tests

import (
	"math"
	"testing"
)

func TestReadInt64_a(t *testing.T) {

	testCases := []struct {
		enc *MixedIntsEnc
		dec *MixedIntsDec
	}{
		{
			enc: &MixedIntsEnc{-13, 57, 32425, math.MaxInt64},
			dec: &MixedIntsDec{-13, 57, 32425, math.MaxInt64},
		},
		{
			enc: &MixedIntsEnc{324, 127, 2, math.MaxInt8},
			dec: &MixedIntsDec{324, 127, 2, math.MaxInt8},
		},
		{
			enc: &MixedIntsEnc{324, 57, 32425, 88},
			dec: &MixedIntsDec{324, 57, 32425, 88},
		},
		{ // For this case, ensure than an error is returned for field B.
			enc: &MixedIntsEnc{324, math.MaxUint8, 5, 88},
			dec: &MixedIntsDec{324, 0, 5, 88},
		},
	}

	for i, tc := range testCases {

		enc, err := tc.enc.MarshalMsg(nil)
		if err != nil {
			t.Fatalf("could not marshall struct (index %d): %s", i, err)
		}

		dec := new(MixedIntsDec)
		_, err = dec.UnmarshalMsg(enc)
		if err != nil {
			if i == 3 {
				continue // There should be an error unmarshalling MaxUint8 (field B) into into an int8 field.
			}
			t.Fatalf("could not unmarshall struct (index %d): %s", i, err)
		}

		if tc.dec.A != dec.A {
			t.Errorf("decoded bad A; got %d", dec.A)
		}
		if tc.dec.B != dec.B {
			t.Errorf("decoded bad B; got %d", dec.B)
		}
		if tc.dec.C != dec.C {
			t.Errorf("decoded bad C; got %d", dec.C)
		}
		if tc.dec.D != dec.D {
			t.Errorf("decoded bad D; got %d", dec.D)
		}

	}

}

func TestReadInt64_b(t *testing.T) {

	testCases := []struct {
		enc Uint16Enc
		dec Int32ForUint16Dec
	}{
		{0, 0},
		{100, 100},
		{math.MaxUint16, math.MaxUint16},
	}

	for i, tc := range testCases {

		enc, err := tc.enc.MarshalMsg(nil)
		if err != nil {
			t.Fatalf("could not marshall number (index %d): %s", i, err)
		}

		dec := new(Int32ForUint16Dec)
		_, err = dec.UnmarshalMsg(enc)
		if err != nil {
			t.Fatalf("could not unmarshall number (index %d): %s", i, err)
		}

		if tc.dec != *dec {
			t.Errorf("decoded bad A; got %d", dec)
		}

	}

}
