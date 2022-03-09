package display

import (
	"strconv"
	"strings"

	"github.com/auth0/go-auth0/management"

	"github.com/auth0/auth0-cli/internal/ansi"
)

type breachedPasswordDetectionView struct {
	Enabled                    string
	Shields                    []string
	AdminNotificationFrequency []string
	Method                     string

	raw interface{}
}

func (bpd *breachedPasswordDetectionView) AsTableHeader() []string {
	return []string{"Enabled", "Shields", "AdminNotificationFrequency", "Method"}
}

func (bpd *breachedPasswordDetectionView) AsTableRow() []string {
	return []string{
		bpd.Enabled,
		strings.Join(bpd.Shields, ", "),
		strings.Join(bpd.AdminNotificationFrequency, ", "),
		bpd.Method,
	}
}

func (bpd *breachedPasswordDetectionView) KeyValues() [][]string {
	return [][]string{
		{ansi.Bold("ENABLED"), bpd.Enabled},
		{ansi.Bold("SHIELDS"), strings.Join(bpd.Shields, ", ")},
		{ansi.Bold("ADMIN_NOTIFICATION_FREQUENCY"), strings.Join(bpd.AdminNotificationFrequency, ", ")},
		{ansi.Bold("METHOD"), bpd.Method},
	}
}

func (bpd *breachedPasswordDetectionView) Object() interface{} {
	return bpd.raw
}

func (r *Renderer) BreachedPasswordDetectionShow(bpd *management.BreachedPasswordDetection) {
	r.Heading("Breached Password Detection")
	r.Result(makeBreachedPasswordDetectionView(bpd))
}

func (r *Renderer) BreachedPasswordDetectionUpdate(bpd *management.BreachedPasswordDetection) {
	r.Heading("Breached Password Detection Updated")
	r.Result(makeBreachedPasswordDetectionView(bpd))
}

func makeBreachedPasswordDetectionView(bpd *management.BreachedPasswordDetection) *breachedPasswordDetectionView {
	return &breachedPasswordDetectionView{
		Enabled:                    boolean(bpd.GetEnabled()),
		Shields:                    bpd.GetShields(),
		AdminNotificationFrequency: bpd.GetAdminNotificationFrequency(),
		Method:                     bpd.GetMethod(),

		raw: bpd,
	}
}

type bruteForceProtectionView struct {
	Enabled     string
	Shields     []string
	AllowList   []string
	Mode        string
	MaxAttempts int

	raw interface{}
}

func (bfp *bruteForceProtectionView) AsTableHeader() []string {
	return []string{"Enabled", "Shields", "AllowList", "Mode", "MaxAttempts"}
}

func (bfp *bruteForceProtectionView) AsTableRow() []string {
	return []string{
		bfp.Enabled,
		strings.Join(bfp.Shields, ", "),
		strings.Join(bfp.AllowList, ", "),
		bfp.Mode,
		strconv.Itoa(bfp.MaxAttempts),
	}
}

func (bfp *bruteForceProtectionView) KeyValues() [][]string {
	return [][]string{
		{ansi.Bold("ENABLED"), bfp.Enabled},
		{ansi.Bold("SHIELDS"), strings.Join(bfp.Shields, ", ")},
		{ansi.Bold("ALLOW_LIST"), strings.Join(bfp.AllowList, ", ")},
		{ansi.Bold("MODE"), bfp.Mode},
		{ansi.Bold("MAX_ATTEMPTS"), strconv.Itoa(bfp.MaxAttempts)},
	}
}

func (bfp *bruteForceProtectionView) Object() interface{} {
	return bfp.raw
}

func (r *Renderer) BruteForceProtectionShow(bfp *management.BruteForceProtection) {
	r.Heading("Brute Force Protection")
	r.Result(makeBruteForceProtectionView(bfp))
}

func (r *Renderer) BruteForceProtectionUpdate(bfp *management.BruteForceProtection) {
	r.Heading("Brute Force Protection Updated")
	r.Result(makeBruteForceProtectionView(bfp))
}

func makeBruteForceProtectionView(bfp *management.BruteForceProtection) *bruteForceProtectionView {
	return &bruteForceProtectionView{
		Enabled:     boolean(bfp.GetEnabled()),
		Shields:     bfp.GetShields(),
		AllowList:   bfp.GetAllowList(),
		Mode:        bfp.GetMode(),
		MaxAttempts: bfp.GetMaxAttempts(),

		raw: bfp,
	}
}

type suspiciousIPThrottlingView struct {
	Enabled                             string
	Shields                             []string
	AllowList                           []string
	StagePreLoginMaxAttempts            int
	StagePreLoginRate                   int
	StagePreUserRegistrationMaxAttempts int
	StagePreUserRegistrationRate        int

	raw interface{}
}

func (sit *suspiciousIPThrottlingView) AsTableHeader() []string {
	return []string{
		"Enabled",
		"Shields",
		"AllowList",
		"StagePreLoginMaxAttempts",
		"StagePreLoginRate",
		"StagePreUserRegistrationMaxAttempts",
		"StagePreUserRegistrationRate",
	}
}

func (sit *suspiciousIPThrottlingView) AsTableRow() []string {
	return []string{
		sit.Enabled,
		strings.Join(sit.Shields, ", "),
		strings.Join(sit.AllowList, ", "),
		strconv.Itoa(sit.StagePreLoginMaxAttempts),
		strconv.Itoa(sit.StagePreLoginRate),
		strconv.Itoa(sit.StagePreUserRegistrationMaxAttempts),
		strconv.Itoa(sit.StagePreUserRegistrationRate),
	}
}

func (sit *suspiciousIPThrottlingView) KeyValues() [][]string {
	return [][]string{
		{ansi.Bold("ENABLED"), sit.Enabled},
		{ansi.Bold("SHIELDS"), strings.Join(sit.Shields, ", ")},
		{ansi.Bold("ALLOW_LIST"), strings.Join(sit.AllowList, ", ")},
		{ansi.Bold("STAGE_PRE_LOGIN_MAX_ATTEMPTS"), strconv.Itoa(sit.StagePreLoginMaxAttempts)},
		{ansi.Bold("STAGE_PRE_LOGIN_RATE"), strconv.Itoa(sit.StagePreLoginRate)},
		{ansi.Bold("STAGE_PRE_USER_REGISTRATION_MAX_ATTEMPTS"), strconv.Itoa(sit.StagePreUserRegistrationMaxAttempts)},
		{ansi.Bold("STAGE_PRE_USER_REGISTRATION_RATE"), strconv.Itoa(sit.StagePreUserRegistrationRate)},
	}
}

func (sit *suspiciousIPThrottlingView) Object() interface{} {
	return sit.raw
}

func (r *Renderer) SuspiciousIPThrottlingShow(sit *management.SuspiciousIPThrottling) {
	r.Heading("Suspicious IP Throttling")
	r.Result(makeSuspiciousIPThrottlingView(sit))
}

func (r *Renderer) SuspiciousIPThrottlingUpdate(sit *management.SuspiciousIPThrottling) {
	r.Heading("Suspicious IP Throttling Updated")
	r.Result(makeSuspiciousIPThrottlingView(sit))
}

func makeSuspiciousIPThrottlingView(sit *management.SuspiciousIPThrottling) *suspiciousIPThrottlingView {
	return &suspiciousIPThrottlingView{
		Enabled:                             boolean(sit.GetEnabled()),
		Shields:                             sit.GetShields(),
		AllowList:                           sit.GetAllowList(),
		StagePreLoginMaxAttempts:            sit.Stage.PreLogin.GetMaxAttempts(),
		StagePreLoginRate:                   sit.Stage.PreLogin.GetRate(),
		StagePreUserRegistrationMaxAttempts: sit.Stage.PreUserRegistration.GetMaxAttempts(),
		StagePreUserRegistrationRate:        sit.Stage.PreUserRegistration.GetRate(),

		raw: sit,
	}
}
