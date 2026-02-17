package display

import (
	"strconv"
	"strings"

	"github.com/auth0/go-auth0/management"
	managementv2 "github.com/auth0/go-auth0/v2/management"

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
	return []string{}
}

func (bpd *breachedPasswordDetectionView) AsTableRow() []string {
	return []string{}
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
	r.Heading("breached password detection")
	r.Result(makeBreachedPasswordDetectionView(bpd))
}

func (r *Renderer) BreachedPasswordDetectionUpdate(bpd *management.BreachedPasswordDetection) {
	r.Heading("breached password detection updated")
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
	return []string{}
}

func (bfp *bruteForceProtectionView) AsTableRow() []string {
	return []string{}
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
	r.Heading("brute force protection")
	r.Result(makeBruteForceProtectionView(bfp))
}

func (r *Renderer) BruteForceProtectionUpdate(bfp *management.BruteForceProtection) {
	r.Heading("brute force protection updated")
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
	return []string{}
}

func (sit *suspiciousIPThrottlingView) AsTableRow() []string {
	return []string{}
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
	r.Heading("suspicious ip throttling")
	r.Result(makeSuspiciousIPThrottlingView(sit))
}

func (r *Renderer) SuspiciousIPThrottlingUpdate(sit *management.SuspiciousIPThrottling) {
	r.Heading("suspicious ip throttling updated")
	r.Result(makeSuspiciousIPThrottlingView(sit))
}

func makeSuspiciousIPThrottlingView(sit *management.SuspiciousIPThrottling) *suspiciousIPThrottlingView {
	view := &suspiciousIPThrottlingView{
		Enabled:   boolean(sit.GetEnabled()),
		Shields:   sit.GetShields(),
		AllowList: sit.GetAllowList(),

		raw: sit,
	}

	if sit.Stage != nil {
		if sit.Stage.PreLogin != nil {
			view.StagePreLoginMaxAttempts = sit.Stage.PreLogin.GetMaxAttempts()
			view.StagePreLoginRate = sit.Stage.PreLogin.GetRate()
		}
		if sit.Stage.PreUserRegistration != nil {
			view.StagePreUserRegistrationMaxAttempts = sit.Stage.PreUserRegistration.GetMaxAttempts()
			view.StagePreUserRegistrationRate = sit.Stage.PreUserRegistration.GetRate()
		}
	}

	return view
}

type botDetectionView struct {
	BotDetectionLevel            string
	ChallengePasswordPolicy      string
	ChallengePasswordlessPolicy  string
	ChallengePasswordResetPolicy string
	AllowList                    []string
	MonitoringModeEnabled        string

	raw interface{}
}

func (bd *botDetectionView) AsTableHeader() []string {
	// There is no list command for this resource, hence this func never gets called.
	// Dummy implementation to satisfy View interface.
	return []string{}
}

func (bd *botDetectionView) AsTableRow() []string {
	// There is no list command for this resource, hence this func never gets called.
	// Dummy implementation to satisfy View interface.
	return []string{}
}

func (bd *botDetectionView) KeyValues() [][]string {
	return [][]string{
		{ansi.Bold("BOT_DETECTION_LEVEL"), bd.BotDetectionLevel},
		{ansi.Bold("CHALLENGE_PASSWORD_POLICY"), bd.ChallengePasswordPolicy},
		{ansi.Bold("CHALLENGE_PASSWORDLESS_POLICY"), bd.ChallengePasswordlessPolicy},
		{ansi.Bold("CHALLENGE_PASSWORD_RESET_POLICY"), bd.ChallengePasswordResetPolicy},
		{ansi.Bold("ALLOW_LIST"), strings.Join(bd.AllowList, ", ")},
		{ansi.Bold("MONITORING_MODE_ENABLED"), bd.MonitoringModeEnabled},
	}
}

func (bd *botDetectionView) Object() interface{} {
	return bd.raw
}

func (r *Renderer) BotDetectionShow(bd *managementv2.GetBotDetectionSettingsResponseContent) {
	r.Heading("bot detection")
	r.Result(makeBotDetectionShowView(bd))
}

func (r *Renderer) BotDetectionUpdate(bd *managementv2.UpdateBotDetectionSettingsResponseContent) {
	r.Heading("bot detection updated")
	r.Result(makeBotDetectionUpdateView(bd))
}

func makeBotDetectionShowView(bd *managementv2.GetBotDetectionSettingsResponseContent) *botDetectionView {
	return &botDetectionView{
		BotDetectionLevel:            string(bd.GetBotDetectionLevel()),
		ChallengePasswordPolicy:      string(bd.GetChallengePasswordPolicy()),
		ChallengePasswordlessPolicy:  string(bd.GetChallengePasswordlessPolicy()),
		ChallengePasswordResetPolicy: string(bd.GetChallengePasswordResetPolicy()),
		AllowList:                    bd.GetAllowlist(),
		MonitoringModeEnabled:        boolean(bd.GetMonitoringModeEnabled()),

		raw: bd,
	}
}

func makeBotDetectionUpdateView(bd *managementv2.UpdateBotDetectionSettingsResponseContent) *botDetectionView {
	return &botDetectionView{
		BotDetectionLevel:            string(bd.GetBotDetectionLevel()),
		ChallengePasswordPolicy:      string(bd.GetChallengePasswordPolicy()),
		ChallengePasswordlessPolicy:  string(bd.GetChallengePasswordlessPolicy()),
		ChallengePasswordResetPolicy: string(bd.GetChallengePasswordResetPolicy()),
		AllowList:                    bd.GetAllowlist(),
		MonitoringModeEnabled:        boolean(bd.GetMonitoringModeEnabled()),

		raw: bd,
	}
}
