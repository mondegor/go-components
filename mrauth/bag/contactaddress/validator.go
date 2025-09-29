package contactaddress

import (
	"regexp"
)

//nolint:lll
var (
	regexpEmail         = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
	regexpPhone         = regexp.MustCompile(`^\+?[0-9][0-9\-. ()]+[0-9]$`)
	regexpPhoneCIS      = regexp.MustCompile(`^(\+?7|8)([348]\d|9[0-6789]|7[0247])\d{8}|\+?(99[^4568]\d{7,11}|994\d{9}|995[57]\d{8}|996[57]\d{8}|998[235789]\d{8}|380[34569]\d{8}|375[234]\d{8}|372\d{7,8}|37[0-4]\d{8})$`)
	regexpPhoneCISNot   = regexp.MustCompile(`^(\+?7|8)(95[4-7]|998|907|94[^09]|336)\d*$`)
	regexpPhoneWorld    = regexp.MustCompile(`^\+?([87]([348]\d|9[0-6789]|7[01247])\d{8}|[1246]\d{9,13}|68\d{7}|5[1-46-9]\d{8,12}|55[1-9]\d{9}|55[138]\d{10}|55[1256][14679]9\d{8}|554399\d{7}|500[56]\d{4}|5016\d{6}|5068\d{7}|502[345]\d{7}|5037\d{7}|50[4567]\d{8}|50855\d{4}|509[34]\d{7}|376\d{6}|855\d{8,9}|856\d{10}|85[0-4789]\d{8,10}|8[68]\d{10,11}|8[14]\d{10}|82\d{9,10}|852\d{8}|90\d{10}|96(0[79]|17[0189]|181|13)\d{6}|96[23]\d{9}|964\d{10}|96(5[569]|89)\d{7}|96(65|77)\d{8}|92[023]\d{9}|91[1879]\d{9}|9[34]7\d{8}|959\d{7,9}|989\d{9}|971\d{8,9}|97[02-9]\d{7,11}|99[^4568]\d{7,11}|994\d{9}|995[57]\d{8}|996[2579]\d{8}|998[235789]\d{8}|380[345679]\d{8}|381\d{9}|38[57]\d{8,9}|375[234]\d{8}|372\d{7,8}|37[0-4]\d{8}|37[6-9]\d{7,11}|30[69]\d{9}|34[679]\d{8}|3459\d{11}|316\d{8}|324\d{8}|33[67]\d{8}|35\d{8,12}|36\d{9}|39\d{8,10}|38[169]\d{8}|382\d{8,9}|46719\d{10})$`)
	regexpPhoneWorldNot = regexp.MustCompile(`^\+?([87](95[5-7]|998|907|94[^09]|336)\d*)$`)
)

// ValidateEmail - comment func.
func ValidateEmail(value string) bool {
	return regexpEmail.MatchString(value)
}

// ValidatePhone - comment func.
func ValidatePhone(value string) bool {
	return regexpPhone.MatchString(value)
}

// ValidatePhoneCIS - comment func.
func ValidatePhoneCIS(value string) bool {
	return regexpPhoneCIS.MatchString(value) && !regexpPhoneCISNot.MatchString(value)
}

// ValidatePhoneWorld - comment func.
func ValidatePhoneWorld(value string) bool {
	return regexpPhoneWorld.MatchString(value) && !regexpPhoneWorldNot.MatchString(value)
}
