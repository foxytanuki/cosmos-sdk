package types

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"testing"
)

var _ CustomProtobufType = (*Dec)(nil)

// NOTE: never use new(Dec) or else we will panic unmarshalling into the
// nil embedded big.Int
type Dec struct {
	i *big.Int
}

const (
	// number of decimal places
	Precision = 18

	// bits required to represent the above precision
	// Ceiling[Log2[10^Precision - 1]]
	DecimalPrecisionBits = 60

	// decimalTruncateBits is the minimum number of bits removed
	// by a truncate operation. It is equal to
	// Floor[Log2[10^Precision - 1]].
	decimalTruncateBits = DecimalPrecisionBits - 1

	maxDecBitLen = maxBitLen + decimalTruncateBits

	// max number of iterations in ApproxRoot function
	maxApproxRootIterations = 100
)

var (
	precisionReuse       = new(big.Int).Exp(big.NewInt(10), big.NewInt(Precision), nil)
	fivePrecision        = new(big.Int).Quo(precisionReuse, big.NewInt(2))
	precisionMultipliers []*big.Int
	zeroInt              = big.NewInt(0)
	oneInt               = big.NewInt(1)
	tenInt               = big.NewInt(10)
)

// Decimal errors
var (
	ErrEmptyDecimalStr      = errors.New("decimal string cannot be empty")
	ErrInvalidDecimalLength = errors.New("invalid decimal length")
	ErrInvalidDecimalStr    = errors.New("invalid decimal string")
)

// Set precision multipliers
func init() {
	precisionMultipliers = make([]*big.Int, Precision+1)
	for i := 0; i <= Precision; i++ {
		precisionMultipliers[i] = calcPrecisionMultiplier(int64(i))
	}
}

func precisionInt() *big.Int {
	return new(big.Int).Set(precisionReuse)
}

func ZeroDec() Dec     { return Dec{new(big.Int).Set(zeroInt)} }
func OneDec() Dec      { return Dec{precisionInt()} }
func SmallestDec() Dec { return Dec{new(big.Int).Set(oneInt)} }

// calculate the precision multiplier
func calcPrecisionMultiplier(prec int64) *big.Int {
	if prec > Precision {
		panic(fmt.Sprintf("too much precision, maximum %v, provided %v", Precision, prec))
	}
	zerosToAdd := Precision - prec
	multiplier := new(big.Int).Exp(tenInt, big.NewInt(zerosToAdd), nil)
	return multiplier
}

// get the precision multiplier, do not mutate result
func precisionMultiplier(prec int64) *big.Int {
	if prec > Precision {
		panic(fmt.Sprintf("too much precision, maximum %v, provided %v", Precision, prec))
	}
	return precisionMultipliers[prec]
}

// create a new Dec from integer assuming whole number
func NewDec(i int64) Dec {
	return NewDecWithPrec(i, 0)
}

// create a new Dec from integer with decimal place at prec
// CONTRACT: prec <= Precision
func NewDecWithPrec(i, prec int64) Dec {
	return Dec{
		new(big.Int).Mul(big.NewInt(i), precisionMultiplier(prec)),
	}
}

// create a new Dec from big integer assuming whole numbers
// CONTRACT: prec <= Precision
func NewDecFromBigInt(i *big.Int) Dec {
	return NewDecFromBigIntWithPrec(i, 0)
}

// create a new Dec from big integer assuming whole numbers
// CONTRACT: prec <= Precision
func NewDecFromBigIntWithPrec(i *big.Int, prec int64) Dec {
	return Dec{
		new(big.Int).Mul(i, precisionMultiplier(prec)),
	}
}

// create a new Dec from big integer assuming whole numbers
// CONTRACT: prec <= Precision
func NewDecFromInt(i Int) Dec {
	return NewDecFromIntWithPrec(i, 0)
}

// create a new Dec from big integer with decimal place at prec
// CONTRACT: prec <= Precision
func NewDecFromIntWithPrec(i Int, prec int64) Dec {
	return Dec{
		new(big.Int).Mul(i.BigInt(), precisionMultiplier(prec)),
	}
}

// create a decimal from an input decimal string.
// valid must come in the form:
//   (-) whole integers (.) decimal integers
// examples of acceptable input include:
//   -123.456
//   456.7890
//   345
//   -456789
//
// NOTE - An error will return if more decimal places
// are provided in the string than the constant Precision.
//
// CONTRACT - This function does not mutate the input str.
func NewDecFromStr(str string) (Dec, error) {
	if len(str) == 0 {
		return Dec{}, ErrEmptyDecimalStr
	}

	// first extract any negative symbol
	neg := false
	if str[0] == '-' {
		neg = true
		str = str[1:]
	}

	if len(str) == 0 {
		return Dec{}, ErrEmptyDecimalStr
	}

	strs := strings.Split(str, ".")
	lenDecs := 0
	combinedStr := strs[0]

	if len(strs) == 2 { // has a decimal place
		lenDecs = len(strs[1])
		if lenDecs == 0 || len(combinedStr) == 0 {
			return Dec{}, ErrInvalidDecimalLength
		}
		combinedStr += strs[1]
	} else if len(strs) > 2 {
		return Dec{}, ErrInvalidDecimalStr
	}

	if lenDecs > Precision {
		return Dec{}, fmt.Errorf("invalid precision; max: %d, got: %d", Precision, lenDecs)
	}

	// add some extra zero's to correct to the Precision factor
	zerosToAdd := Precision - lenDecs
	zeros := fmt.Sprintf(`%0`+strconv.Itoa(zerosToAdd)+`s`, "")
	combinedStr += zeros

	combined, ok := new(big.Int).SetString(combinedStr, 10) // base 10
	if !ok {
		return Dec{}, fmt.Errorf("failed to set decimal string: %s", combinedStr)
	}
	if combined.BitLen() > maxDecBitLen {
		return Dec{}, fmt.Errorf("decimal out of range; bitLen: got %d, max %d", combined.BitLen(), maxDecBitLen)
	}
	if neg {
		combined = new(big.Int).Neg(combined)
	}

	return Dec{combined}, nil
}

// Decimal from string, panic on error
func MustNewDecFromStr(s string) Dec {
	dec, err := NewDecFromStr(s)
	if err != nil {
		panic(err)
	}
	return dec
}

func (d Dec) IsNil() bool       { return d.i == nil }                 // is decimal nil
func (d Dec) IsZero() bool      { return (d.i).Sign() == 0 }          // is equal to zero
func (d Dec) IsNegative() bool  { return (d.i).Sign() == -1 }         // is negative
func (d Dec) IsPositive() bool  { return (d.i).Sign() == 1 }          // is positive
func (d Dec) Equal(d2 Dec) bool { return (d.i).Cmp(d2.i) == 0 }       // equal decimals
func (d Dec) GT(d2 Dec) bool    { return (d.i).Cmp(d2.i) > 0 }        // greater than
func (d Dec) GTE(d2 Dec) bool   { return (d.i).Cmp(d2.i) >= 0 }       // greater than or equal
func (d Dec) LT(d2 Dec) bool    { return (d.i).Cmp(d2.i) < 0 }        // less than
func (d Dec) LTE(d2 Dec) bool   { return (d.i).Cmp(d2.i) <= 0 }       // less than or equal
func (d Dec) Neg() Dec          { return Dec{new(big.Int).Neg(d.i)} } // reverse the decimal sign
func (d Dec) NegMut() Dec       { d.i.Neg(d.i); return d }            // reverse the decimal sign, mutable
func (d Dec) Abs() Dec          { return Dec{new(big.Int).Abs(d.i)} } // absolute value
func (d Dec) Set(d2 Dec) Dec    { d.i.Set(d2.i); return d }           // set to existing dec value
func (d Dec) Clone() Dec        { return Dec{new(big.Int).Set(d.i)} } // clone new dec

// BigInt returns a copy of the underlying big.Int.
func (d Dec) BigInt() *big.Int {
	if d.IsNil() {
		return nil
	}

	cp := new(big.Int)
	return cp.Set(d.i)
}

func (d Dec) ImmutOp(op func(Dec, Dec) Dec, d2 Dec) Dec {
	return op(d.Clone(), d2)
}

func (d Dec) ImmutOpInt(op func(Dec, Int) Dec, d2 Int) Dec {
	return op(d.Clone(), d2)
}

func (d Dec) ImmutOpInt64(op func(Dec, int64) Dec, d2 int64) Dec {
	// TODO: use already allocated operand bigint to avoid
	// newint each time, add mutex for race condition
	// Issue: https://github.com/cosmos/cosmos-sdk/issues/11166
	return op(d.Clone(), d2)
}

func (d Dec) SetInt64(i int64) Dec {
	d.i.SetInt64(i)
	d.i.Mul(d.i, precisionReuse)
	return d
}

// addition
func (d Dec) Add(d2 Dec) Dec {
	return d.ImmutOp(Dec.AddMut, d2)
}

// mutable addition
func (d Dec) AddMut(d2 Dec) Dec {
	d.i.Add(d.i, d2.i)

	if d.i.BitLen() > maxDecBitLen {
		panic("Int overflow")
	}
	return d
}

// subtraction
func (d Dec) Sub(d2 Dec) Dec {
	return d.ImmutOp(Dec.SubMut, d2)
}

// mutable subtraction
func (d Dec) SubMut(d2 Dec) Dec {
	d.i.Sub(d.i, d2.i)

	if d.i.BitLen() > maxDecBitLen {
		panic("Int overflow")
	}
	return d
}

// multiplication
func (d Dec) Mul(d2 Dec) Dec {
	return d.ImmutOp(Dec.MulMut, d2)
}

// mutable multiplication
func (d Dec) MulMut(d2 Dec) Dec {
	d.i.Mul(d.i, d2.i)
	chopped := chopPrecisionAndRound(d.i)

	if chopped.BitLen() > maxDecBitLen {
		panic("Int overflow")
	}
	*d.i = *chopped
	return d
}

// multiplication truncate
func (d Dec) MulTruncate(d2 Dec) Dec {
	return d.ImmutOp(Dec.MulTruncateMut, d2)
}

// mutable multiplication truncage
func (d Dec) MulTruncateMut(d2 Dec) Dec {
	d.i.Mul(d.i, d2.i)
	chopPrecisionAndTruncate(d.i)

	if d.i.BitLen() > maxDecBitLen {
		panic("Int overflow")
	}
	return d
}

// multiplication
func (d Dec) MulInt(i Int) Dec {
	return d.ImmutOpInt(Dec.MulIntMut, i)
}

func (d Dec) MulIntMut(i Int) Dec {
	d.i.Mul(d.i, i.i)
	if d.i.BitLen() > maxDecBitLen {
		panic("Int overflow")
	}
	return d
}

// MulInt64 - multiplication with int64
func (d Dec) MulInt64(i int64) Dec {
	return d.ImmutOpInt64(Dec.MulInt64Mut, i)
}

func (d Dec) MulInt64Mut(i int64) Dec {
	d.i.Mul(d.i, big.NewInt(i))

	if d.i.BitLen() > maxDecBitLen {
		panic("Int overflow")
	}
	return d
}

// quotient
func (d Dec) Quo(d2 Dec) Dec {
	return d.ImmutOp(Dec.QuoMut, d2)
}

// mutable quotient
func (d Dec) QuoMut(d2 Dec) Dec {
	// multiply precision twice
	d.i.Mul(d.i, precisionReuse)
	d.i.Mul(d.i, precisionReuse)
	d.i.Quo(d.i, d2.i)

	chopPrecisionAndRound(d.i)
	if d.i.BitLen() > maxDecBitLen {
		panic("Int overflow")
	}
	return d
}

// quotient truncate
func (d Dec) QuoTruncate(d2 Dec) Dec {
	return d.ImmutOp(Dec.QuoTruncateMut, d2)
}

// mutable quotient truncate
func (d Dec) QuoTruncateMut(d2 Dec) Dec {
	// multiply precision twice
	d.i.Mul(d.i, precisionReuse)
	d.i.Mul(d.i, precisionReuse)
	d.i.Quo(d.i, d2.i)

	chopPrecisionAndTruncate(d.i)
	if d.i.BitLen() > maxDecBitLen {
		panic("Int overflow")
	}
	return d
}

// quotient, round up
func (d Dec) QuoRoundUp(d2 Dec) Dec {
	return d.ImmutOp(Dec.QuoRoundupMut, d2)
}

// mutable quotient, round up
func (d Dec) QuoRoundupMut(d2 Dec) Dec {
	// multiply precision twice
	d.i.Mul(d.i, precisionReuse)
	d.i.Mul(d.i, precisionReuse)
	d.i.Quo(d.i, d2.i)

	chopPrecisionAndRoundUp(d.i)
	if d.i.BitLen() > maxDecBitLen {
		panic("Int overflow")
	}
	return d
}

// quotient
func (d Dec) QuoInt(i Int) Dec {
	return d.ImmutOpInt(Dec.QuoIntMut, i)
}

func (d Dec) QuoIntMut(i Int) Dec {
	d.i.Quo(d.i, i.i)
	return d
}

// QuoInt64 - quotient with int64
func (d Dec) QuoInt64(i int64) Dec {
	return d.ImmutOpInt64(Dec.QuoInt64Mut, i)
}

func (d Dec) QuoInt64Mut(i int64) Dec {
	d.i.Quo(d.i, big.NewInt(i))
	return d
}

// ApproxRoot returns an approximate estimation of a Dec's positive real nth root
// using Newton's method (where n is positive). The algorithm starts with some guess and
// computes the sequence of improved guesses until an answer converges to an
// approximate answer.  It returns `|d|.ApproxRoot() * -1` if input is negative.
// A maximum number of 100 iterations is used a backup boundary condition for
// cases where the answer never converges enough to satisfy the main condition.
func (d Dec) ApproxRoot(root uint64) (guess Dec, err error) {
	defer func() {
		if r := recover(); r != nil {
			var ok bool
			err, ok = r.(error)
			if !ok {
				err = errors.New("out of bounds")
			}
		}
	}()

	if d.IsNegative() {
		absRoot, err := d.Neg().ApproxRoot(root)
		return absRoot.NegMut(), err
	}

	if root == 1 || d.IsZero() || d.Equal(OneDec()) {
		return d, nil
	}

	if root == 0 {
		return OneDec(), nil
	}

	guess, delta := OneDec(), OneDec()

	for iter := 0; delta.Abs().GT(SmallestDec()) && iter < maxApproxRootIterations; iter++ {
		prev := guess.Power(root - 1)
		if prev.IsZero() {
			prev = SmallestDec()
		}
		delta.Set(d).QuoMut(prev)
		delta.SubMut(guess)
		delta.QuoInt64Mut(int64(root))

		guess.AddMut(delta)
	}

	return guess, nil
}

// Power returns a the result of raising to a positive integer power
func (d Dec) Power(power uint64) Dec {
	res := Dec{new(big.Int).Set(d.i)}
	return res.PowerMut(power)
}

func (d Dec) PowerMut(power uint64) Dec {
	if power == 0 {
		d.SetInt64(1)
		return d
	}
	tmp := OneDec()

	for i := power; i > 1; {
		if i%2 != 0 {
			tmp.MulMut(d)
		}
		i /= 2
		d.MulMut(d)
	}

	return d.MulMut(tmp)
}

// ApproxSqrt is a wrapper around ApproxRoot for the common special case
// of finding the square root of a number. It returns -(sqrt(abs(d)) if input is negative.
func (d Dec) ApproxSqrt() (Dec, error) {
	return d.ApproxRoot(2)
}

// is integer, e.g. decimals are zero
func (d Dec) IsInteger() bool {
	return new(big.Int).Rem(d.i, precisionReuse).Sign() == 0
}

// format decimal state
func (d Dec) Format(s fmt.State, verb rune) {
	_, err := s.Write([]byte(d.String()))
	if err != nil {
		panic(err)
	}
}

func (d Dec) String() string {
	if d.i == nil {
		return d.i.String()
	}

	isNeg := d.IsNegative()

	if isNeg {
		d = d.Neg()
	}

	bzInt, err := d.i.MarshalText()
	if err != nil {
		return ""
	}
	inputSize := len(bzInt)

	var bzStr []byte

	// TODO: Remove trailing zeros
	// case 1, purely decimal
	if inputSize <= Precision {
		bzStr = make([]byte, Precision+2)

		// 0. prefix
		bzStr[0] = byte('0')
		bzStr[1] = byte('.')

		// set relevant digits to 0
		for i := 0; i < Precision-inputSize; i++ {
			bzStr[i+2] = byte('0')
		}

		// set final digits
		copy(bzStr[2+(Precision-inputSize):], bzInt)
	} else {
		// inputSize + 1 to account for the decimal point that is being added
		bzStr = make([]byte, inputSize+1)
		decPointPlace := inputSize - Precision

		copy(bzStr, bzInt[:decPointPlace])                   // pre-decimal digits
		bzStr[decPointPlace] = byte('.')                     // decimal point
		copy(bzStr[decPointPlace+1:], bzInt[decPointPlace:]) // post-decimal digits
	}

	if isNeg {
		return "-" + string(bzStr)
	}

	return string(bzStr)
}

// Float64 returns the float64 representation of a Dec.
// Will return the error if the conversion failed.
func (d Dec) Float64() (float64, error) {
	return strconv.ParseFloat(d.String(), 64)
}

// MustFloat64 returns the float64 representation of a Dec.
// Would panic if the conversion failed.
func (d Dec) MustFloat64() float64 {
	if value, err := strconv.ParseFloat(d.String(), 64); err != nil {
		panic(err)
	} else {
		return value
	}
}

//     ____
//  __|    |__   "chop 'em
//       ` \     round!"
// ___||  ~  _     -bankers
// |         |      __
// |       | |   __|__|__
// |_____:  /   | $$$    |
//              |________|

// Remove a Precision amount of rightmost digits and perform bankers rounding
// on the remainder (gaussian rounding) on the digits which have been removed.
//
// Mutates the input. Use the non-mutative version if that is undesired
func chopPrecisionAndRound(d *big.Int) *big.Int {
	// remove the negative and add it back when returning
	if d.Sign() == -1 {
		// make d positive, compute chopped value, and then un-mutate d
		d = d.Neg(d)
		d = chopPrecisionAndRound(d)
		d = d.Neg(d)
		return d
	}

	// get the truncated quotient and remainder
	quo, rem := d, big.NewInt(0)
	quo, rem = quo.QuoRem(d, precisionReuse, rem)

	if rem.Sign() == 0 { // remainder is zero
		return quo
	}

	switch rem.Cmp(fivePrecision) {
	case -1:
		return quo
	case 1:
		return quo.Add(quo, oneInt)
	default: // bankers rounding must take place
		// always round to an even number
		if quo.Bit(0) == 0 {
			return quo
		}
		return quo.Add(quo, oneInt)
	}
}

func chopPrecisionAndRoundUp(d *big.Int) *big.Int {
	// remove the negative and add it back when returning
	if d.Sign() == -1 {
		// make d positive, compute chopped value, and then un-mutate d
		d = d.Neg(d)
		// truncate since d is negative...
		chopPrecisionAndTruncate(d)
		d = d.Neg(d)
		return d
	}

	// get the truncated quotient and remainder
	quo, rem := d, big.NewInt(0)
	quo, rem = quo.QuoRem(d, precisionReuse, rem)

	if rem.Sign() == 0 { // remainder is zero
		return quo
	}

	return quo.Add(quo, oneInt)
}

func chopPrecisionAndRoundNonMutative(d *big.Int) *big.Int {
	tmp := new(big.Int).Set(d)
	return chopPrecisionAndRound(tmp)
}

// RoundInt64 rounds the decimal using bankers rounding
func (d Dec) RoundInt64() int64 {
	chopped := chopPrecisionAndRoundNonMutative(d.i)
	if !chopped.IsInt64() {
		panic("Int64() out of bound")
	}
	return chopped.Int64()
}

// RoundInt round the decimal using bankers rounding
func (d Dec) RoundInt() Int {
	return NewIntFromBigInt(chopPrecisionAndRoundNonMutative(d.i))
}

// chopPrecisionAndTruncate is similar to chopPrecisionAndRound,
// but always rounds down. It does not mutate the input.
func chopPrecisionAndTruncate(d *big.Int) {
	d.Quo(d, precisionReuse)
}

func chopPrecisionAndTruncateNonMutative(d *big.Int) *big.Int {
	tmp := new(big.Int).Set(d)
	chopPrecisionAndTruncate(tmp)
	return tmp
}

// TruncateInt64 truncates the decimals from the number and returns an int64
func (d Dec) TruncateInt64() int64 {
	chopped := chopPrecisionAndTruncateNonMutative(d.i)
	if !chopped.IsInt64() {
		panic("Int64() out of bound")
	}
	return chopped.Int64()
}

// TruncateInt truncates the decimals from the number and returns an Int
func (d Dec) TruncateInt() Int {
	return NewIntFromBigInt(chopPrecisionAndTruncateNonMutative(d.i))
}

// TruncateDec truncates the decimals from the number and returns a Dec
func (d Dec) TruncateDec() Dec {
	return NewDecFromBigInt(chopPrecisionAndTruncateNonMutative(d.i))
}

// Ceil returns the smallest interger value (as a decimal) that is greater than
// or equal to the given decimal.
func (d Dec) Ceil() Dec {
	tmp := new(big.Int).Set(d.i)

	quo, rem := tmp, big.NewInt(0)
	quo, rem = quo.QuoRem(tmp, precisionReuse, rem)

	// no need to round with a zero remainder regardless of sign
	if rem.Cmp(zeroInt) == 0 {
		return NewDecFromBigInt(quo)
	}

	if rem.Sign() == -1 {
		return NewDecFromBigInt(quo)
	}

	return NewDecFromBigInt(quo.Add(quo, oneInt))
}

// MaxSortableDec is the largest Dec that can be passed into SortableDecBytes()
// Its negative form is the least Dec that can be passed in.
var MaxSortableDec Dec

func init() {
	MaxSortableDec = OneDec().Quo(SmallestDec())
}

// ValidSortableDec ensures that a Dec is within the sortable bounds,
// a Dec can't have a precision of less than 10^-18.
// Max sortable decimal was set to the reciprocal of SmallestDec.
func ValidSortableDec(dec Dec) bool {
	return dec.Abs().LTE(MaxSortableDec)
}

// SortableDecBytes returns a byte slice representation of a Dec that can be sorted.
// Left and right pads with 0s so there are 18 digits to left and right of the decimal point.
// For this reason, there is a maximum and minimum value for this, enforced by ValidSortableDec.
func SortableDecBytes(dec Dec) []byte {
	if !ValidSortableDec(dec) {
		panic("dec must be within bounds")
	}
	// Instead of adding an extra byte to all sortable decs in order to handle max sortable, we just
	// makes its bytes be "max" which comes after all numbers in ASCIIbetical order
	if dec.Equal(MaxSortableDec) {
		return []byte("max")
	}
	// For the same reason, we make the bytes of minimum sortable dec be --, which comes before all numbers.
	if dec.Equal(MaxSortableDec.Neg()) {
		return []byte("--")
	}
	// We move the negative sign to the front of all the left padded 0s, to make negative numbers come before positive numbers
	if dec.IsNegative() {
		return append([]byte("-"), []byte(fmt.Sprintf(fmt.Sprintf("%%0%ds", Precision*2+1), dec.Abs().String()))...)
	}
	return []byte(fmt.Sprintf(fmt.Sprintf("%%0%ds", Precision*2+1), dec.String()))
}

// reuse nil values
var nilJSON []byte

func init() {
	empty := new(big.Int)
	bz, _ := empty.MarshalText()
	nilJSON, _ = json.Marshal(string(bz))
}

// MarshalJSON marshals the decimal
func (d Dec) MarshalJSON() ([]byte, error) {
	if d.i == nil {
		return nilJSON, nil
	}
	return json.Marshal(d.String())
}

// UnmarshalJSON defines custom decoding scheme
func (d *Dec) UnmarshalJSON(bz []byte) error {
	if d.i == nil {
		d.i = new(big.Int)
	}

	var text string
	err := json.Unmarshal(bz, &text)
	if err != nil {
		return err
	}

	// TODO: Reuse dec allocation
	newDec, err := NewDecFromStr(text)
	if err != nil {
		return err
	}

	d.i = newDec.i
	return nil
}

// MarshalYAML returns the YAML representation.
func (d Dec) MarshalYAML() (interface{}, error) {
	return d.String(), nil
}

// Marshal implements the gogo proto custom type interface.
func (d Dec) Marshal() ([]byte, error) {
	if d.i == nil {
		d.i = new(big.Int)
	}
	return d.i.MarshalText()
}

// MarshalTo implements the gogo proto custom type interface.
func (d *Dec) MarshalTo(data []byte) (n int, err error) {
	if d.i == nil {
		d.i = new(big.Int)
	}

	if d.i.Cmp(zeroInt) == 0 {
		copy(data, []byte{0x30})
		return 1, nil
	}

	bz, err := d.Marshal()
	if err != nil {
		return 0, err
	}

	copy(data, bz)
	return len(bz), nil
}

// Unmarshal implements the gogo proto custom type interface.
func (d *Dec) Unmarshal(data []byte) error {
	if len(data) == 0 {
		d = nil
		return nil
	}

	if d.i == nil {
		d.i = new(big.Int)
	}

	if err := d.i.UnmarshalText(data); err != nil {
		return err
	}

	if d.i.BitLen() > maxDecBitLen {
		return fmt.Errorf("decimal out of range; got: %d, max: %d", d.i.BitLen(), maxDecBitLen)
	}

	return nil
}

// Size implements the gogo proto custom type interface.
func (d *Dec) Size() int {
	bz, _ := d.Marshal()
	return len(bz)
}

// Override Amino binary serialization by proxying to protobuf.
func (d Dec) MarshalAmino() ([]byte, error)   { return d.Marshal() }
func (d *Dec) UnmarshalAmino(bz []byte) error { return d.Unmarshal(bz) }

func (dp DecProto) String() string {
	return dp.Dec.String()
}

// helpers

// test if two decimal arrays are equal
func DecsEqual(d1s, d2s []Dec) bool {
	if len(d1s) != len(d2s) {
		return false
	}

	for i, d1 := range d1s {
		if !d1.Equal(d2s[i]) {
			return false
		}
	}
	return true
}

// minimum decimal between two
func MinDec(d1, d2 Dec) Dec {
	if d1.LT(d2) {
		return d1
	}
	return d2
}

// maximum decimal between two
func MaxDec(d1, d2 Dec) Dec {
	if d1.LT(d2) {
		return d2
	}
	return d1
}

// intended to be used with require/assert:  require.True(DecEq(...))
func DecEq(t *testing.T, exp, got Dec) (*testing.T, bool, string, string, string) {
	return t, exp.Equal(got), "expected:\t%v\ngot:\t\t%v", exp.String(), got.String()
}

func DecApproxEq(t *testing.T, d1 Dec, d2 Dec, tol Dec) (*testing.T, bool, string, string, string) {
	diff := d1.Sub(d2).Abs()
	return t, diff.LTE(tol), "expected |d1 - d2| <:\t%v\ngot |d1 - d2| = \t\t%v", tol.String(), diff.String()
}
