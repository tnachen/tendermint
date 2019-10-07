package lite

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/tendermint/tendermint/types"
)

func TestVerifyAdjustedHeaders(t *testing.T) {
	const (
		chainID    = "TestVerifyAdjustedHeaders"
		lastHeight = 1
		nextHeight = 2
	)

	var (
		keys = genPrivKeys(4)
		// 20, 30, 40, 50 - the first 3 don't have 2/3, the last 3 do!
		vals     = keys.ToValidators(20, 10)
		bTime, _ = time.Parse(time.RFC3339, "2006-01-02T15:04:05Z")
		header   = keys.GenSignedHeader(chainID, lastHeight, bTime, nil, vals, vals,
			[]byte("app_hash"), []byte("cons_hash"), []byte("results_hash"), 0, len(keys))
	)

	testCases := []struct {
		newHeader      *types.SignedHeader
		newVals        *types.ValidatorSet
		trustingPeriod time.Duration
		now            time.Time
		expErr         error
		expErrText     string
	}{
		// same header -> no error
		0: {
			header,
			vals,
			3 * time.Hour,
			bTime.Add(2 * time.Hour),
			nil,
			"expected new header height 1 to be greater than one of old header 1",
		},
		// different chainID -> error
		1: {
			keys.GenSignedHeader("different-chainID", nextHeight, bTime.Add(1*time.Hour), nil, vals, vals,
				[]byte("app_hash"), []byte("cons_hash"), []byte("results_hash"), 0, len(keys)),
			vals,
			3 * time.Hour,
			bTime.Add(2 * time.Hour),
			nil,
			"h2.ValidateBasic failed: signedHeader belongs to another chain 'different-chainID' not 'TestVerifyAdjustedHeaders'",
		},
		// 3/3 signed -> no error
		2: {
			keys.GenSignedHeader(chainID, nextHeight, bTime.Add(1*time.Hour), nil, vals, vals,
				[]byte("app_hash"), []byte("cons_hash"), []byte("results_hash"), 0, len(keys)),
			vals,
			3 * time.Hour,
			bTime.Add(2 * time.Hour),
			nil,
			"",
		},
		// 2/3 signed -> no error
		3: {
			keys.GenSignedHeader(chainID, nextHeight, bTime.Add(1*time.Hour), nil, vals, vals,
				[]byte("app_hash"), []byte("cons_hash"), []byte("results_hash"), 1, len(keys)),
			vals,
			3 * time.Hour,
			bTime.Add(2 * time.Hour),
			nil,
			"",
		},
		// 1/3 signed -> error
		4: {
			keys.GenSignedHeader(chainID, nextHeight, bTime.Add(1*time.Hour), nil, vals, vals,
				[]byte("app_hash"), []byte("cons_hash"), []byte("results_hash"), len(keys)-1, len(keys)),
			vals,
			3 * time.Hour,
			bTime.Add(2 * time.Hour),
			types.ErrTooMuchChange{Got: 50.0, Needed: 93.00},
			"",
		},
		// vals does not match with what we have -> error
		5: {
			keys.GenSignedHeader(chainID, nextHeight, bTime.Add(1*time.Hour), nil, keys.ToValidators(10, 1), vals,
				[]byte("app_hash"), []byte("cons_hash"), []byte("results_hash"), 0, len(keys)),
			keys.ToValidators(10, 1),
			3 * time.Hour,
			bTime.Add(2 * time.Hour),
			nil,
			"to match those from new header",
		},
		// vals are inconsistent with newHeader -> error
		6: {
			keys.GenSignedHeader(chainID, nextHeight, bTime.Add(1*time.Hour), nil, vals, vals,
				[]byte("app_hash"), []byte("cons_hash"), []byte("results_hash"), 0, len(keys)),
			keys.ToValidators(10, 1),
			3 * time.Hour,
			bTime.Add(2 * time.Hour),
			nil,
			"to match those that were supplied",
		},
		// old header has expired -> error
		7: {
			keys.GenSignedHeader(chainID, nextHeight, bTime.Add(1*time.Hour), nil, vals, vals,
				[]byte("app_hash"), []byte("cons_hash"), []byte("results_hash"), 0, len(keys)),
			keys.ToValidators(10, 1),
			1 * time.Hour,
			bTime.Add(1 * time.Hour),
			nil,
			"old header has expired",
		},
		// new header is too far into the future -> error
		8: {
			keys.GenSignedHeader(chainID, nextHeight, bTime.Add(4*time.Hour), nil, vals, vals,
				[]byte("app_hash"), []byte("cons_hash"), []byte("results_hash"), 0, len(keys)),
			keys.ToValidators(10, 1),
			3 * time.Hour,
			bTime.Add(2 * time.Hour), // not relevant
			ErrNewHeaderTooFarIntoFuture{bTime.Add(4 * time.Hour), bTime.Add(3 * time.Hour)},
			"",
		},
	}

	for i, tc := range testCases {
		tc := tc
		t.Run(fmt.Sprintf("#%d", i), func(t *testing.T) {
			err := Verify(chainID, header, vals, tc.newHeader, tc.newVals, tc.trustingPeriod, tc.now, DefaultTrustLevel)

			switch {
			case tc.expErr != nil && assert.Error(t, err):
				assert.Equal(t, tc.expErr, err)
			case tc.expErrText != "":
				assert.Contains(t, err.Error(), tc.expErrText)
			default:
				assert.NoError(t, err)
			}
		})
	}
}

func TestVerifyNonAdjustedHeaders(t *testing.T) {
	const (
		chainID    = "TestVerifyNonAdjustedHeaders"
		lastHeight = 1
	)

	var (
		keys = genPrivKeys(4)
		// 20, 30, 40, 50 - the first 3 don't have 2/3, the last 3 do!
		vals     = keys.ToValidators(20, 10)
		bTime, _ = time.Parse(time.RFC3339, "2006-01-02T15:04:05Z")
		header   = keys.GenSignedHeader(chainID, lastHeight, bTime, nil, vals, vals,
			[]byte("app_hash"), []byte("cons_hash"), []byte("results_hash"), 0, len(keys))

		// 30, 40, 50
		twoThirds     = keys[1:]
		twoThirdsVals = twoThirds.ToValidators(30, 10)

		// 50
		oneThird     = keys[len(keys)-1:]
		oneThirdVals = oneThird.ToValidators(50, 10)

		// 20
		lessThanOneThird     = keys[0:1]
		lessThanOneThirdVals = lessThanOneThird.ToValidators(20, 10)
	)

	testCases := []struct {
		newHeader      *types.SignedHeader
		newVals        *types.ValidatorSet
		trustingPeriod time.Duration
		now            time.Time
		expErr         error
		expErrText     string
	}{
		// 3/3 new vals signed, 3/3 old vals present -> no error
		0: {
			keys.GenSignedHeader(chainID, 3, bTime.Add(1*time.Hour), nil, vals, vals,
				[]byte("app_hash"), []byte("cons_hash"), []byte("results_hash"), 0, len(keys)),
			vals,
			3 * time.Hour,
			bTime.Add(2 * time.Hour),
			nil,
			"",
		},
		// 2/3 new vals signed, 3/3 old vals present -> no error
		1: {
			keys.GenSignedHeader(chainID, 4, bTime.Add(1*time.Hour), nil, vals, vals,
				[]byte("app_hash"), []byte("cons_hash"), []byte("results_hash"), 1, len(keys)),
			vals,
			3 * time.Hour,
			bTime.Add(2 * time.Hour),
			nil,
			"",
		},
		// 1/3 new vals signed, 3/3 old vals present -> error
		2: {
			keys.GenSignedHeader(chainID, 5, bTime.Add(1*time.Hour), nil, vals, vals,
				[]byte("app_hash"), []byte("cons_hash"), []byte("results_hash"), len(keys)-1, len(keys)),
			vals,
			3 * time.Hour,
			bTime.Add(2 * time.Hour),
			types.ErrTooMuchChange{Got: 50.0, Needed: 93.00},
			"",
		},
		// 3/3 new vals signed, 2/3 old vals present -> no error
		3: {
			twoThirds.GenSignedHeader(chainID, 5, bTime.Add(1*time.Hour), nil, twoThirdsVals, twoThirdsVals,
				[]byte("app_hash"), []byte("cons_hash"), []byte("results_hash"), 0, len(twoThirds)),
			twoThirdsVals,
			3 * time.Hour,
			bTime.Add(2 * time.Hour),
			nil,
			"",
		},
		// 3/3 new vals signed, 1/3 old vals present -> no error
		4: {
			oneThird.GenSignedHeader(chainID, 5, bTime.Add(1*time.Hour), nil, oneThirdVals, oneThirdVals,
				[]byte("app_hash"), []byte("cons_hash"), []byte("results_hash"), 0, len(oneThird)),
			oneThirdVals,
			3 * time.Hour,
			bTime.Add(2 * time.Hour),
			nil,
			"",
		},
		// 3/3 new vals signed, less than 1/3 old vals present -> error
		5: {
			lessThanOneThird.GenSignedHeader(chainID, 5, bTime.Add(1*time.Hour), nil, lessThanOneThirdVals, lessThanOneThirdVals,
				[]byte("app_hash"), []byte("cons_hash"), []byte("results_hash"), 0, len(lessThanOneThird)),
			lessThanOneThirdVals,
			3 * time.Hour,
			bTime.Add(2 * time.Hour),
			types.ErrTooMuchChange{Got: 20.0, Needed: 46.666668},
			"",
		},
	}

	for i, tc := range testCases {
		tc := tc
		t.Run(fmt.Sprintf("#%d", i), func(t *testing.T) {
			err := Verify(chainID, header, vals, tc.newHeader, tc.newVals, tc.trustingPeriod, tc.now, DefaultTrustLevel)

			switch {
			case tc.expErr != nil && assert.Error(t, err):
				assert.Equal(t, tc.expErr, err)
			case tc.expErrText != "":
				assert.Contains(t, err.Error(), tc.expErrText)
			default:
				assert.NoError(t, err)
			}
		})
	}
}