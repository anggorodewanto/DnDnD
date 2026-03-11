package refdata

// srdCreaturesMidCR returns SRD creatures with CR 3-7.
func srdCreaturesMidCR() []cr {
	return append(srdCreaturesCR3to5(), srdCreaturesCR6to7()...)
}
