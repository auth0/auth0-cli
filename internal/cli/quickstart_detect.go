package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// DetectionResult holds the values resolved by scanning the working directory.
// Fields are empty/zero when not detected. AmbiguousFrameworks is populated when
// multiple package.json deps match and the framework cannot be determined uniquely.
type DetectionResult struct {
	Framework           string
	Type                string   // "spa" | "regular" | "native".
	BuildTool           string   // "vite" | "maven" | "gradle" | "composer" | "" (NA).
	BundleID            string   // Package/bundle ID for native apps (e.g. "com.example.myapp"); empty if not found.
	Detected            bool     // True if any signal file matched.
	AmbiguousFrameworks []string // Set when >1 package.json dep matched.
}

// detectionCandidate is used internally during package.json dep scanning.
type detectionCandidate struct {
	framework string
	qsType    string
	buildTool string
}

// DetectProject scans dir for framework signal files and returns a DetectionResult.
// Rules follow the priority order from the spec: config files beat package.json scanning.
func DetectProject(dir string) DetectionResult {
	result := DetectionResult{}

	// Read package.json deps early - needed for checks that must precede file-based signals.
	earlyDeps := readPackageJSONDeps(dir)

	// -- 1. manage.py (Django) — checked before Ionic to prevent monorepo misdetection.
	if fileExists(dir, "manage.py") {
		result.Framework = "django"
		result.Type = "regular"
		result.Detected = true
		return result
	}

	// -- 2. Ionic (package.json deps - must check BEFORE angular.json and vite.config) --.
	if hasDep(earlyDeps, "@ionic/angular") {
		result.Framework = "ionic-angular"
		result.Type = "native"
		result.BundleID = readCapacitorAppID(dir)
		result.Detected = true
		return result
	}
	if hasDep(earlyDeps, "@ionic/react") {
		result.Framework = "ionic-react"
		result.Type = "native"
		result.BuildTool = detectBuildToolFromFiles(dir, "ionic-react")
		result.BundleID = readCapacitorAppID(dir)
		result.Detected = true
		return result
	}
	if hasDep(earlyDeps, "@ionic/vue") {
		result.Framework = "ionic-vue"
		result.Type = "native"
		result.BuildTool = detectBuildToolFromFiles(dir, "ionic-vue")
		result.BundleID = readCapacitorAppID(dir)
		result.Detected = true
		return result
	}

	// -- 3. Angular.json --.
	if fileExists(dir, "angular.json") {
		result.Framework = "angular"
		result.Type = "spa"
		result.Detected = true
		return result
	}

	// -- 4. pubspec.yaml (Flutter) --.
	if data, ok := readFileContent(dir, "pubspec.yaml"); ok {
		if strings.Contains(data, "sdk: flutter") {
			result.Detected = true
			// android/ios dirs signal native; flutter.web key signals web SPA; default native.
			switch {
			case dirExists(dir, "android") || dirExists(dir, "ios"):
				result.Framework = "flutter"
				result.Type = "native"
				result.BundleID = readMobileBundleID(dir)
			case pubspecHasFlutterWebTarget(data):
				result.Framework = "flutter-web"
				result.Type = "spa"
			default:
				// No native platform dirs and no web target in pubspec - default to native.
				result.Framework = "flutter"
				result.Type = "native"
				result.BundleID = readMobileBundleID(dir)
			}
			return result
		}
	}

	// -- 5. composer.json (PHP) — before vite.config to avoid Laravel misdetection.
	if data, ok := readFileContent(dir, "composer.json"); ok {
		result.BuildTool = "composer"
		result.Type = "regular"
		result.Detected = true
		if strings.Contains(data, "laravel/framework") {
			result.Framework = "laravel"
		} else {
			result.Framework = "vanilla-php"
		}
		return result
	}

	// -- 6. SvelteKit (@sveltejs/kit dep) — before vite.config; only reliable distinguishing signal.
	if hasDep(earlyDeps, "@sveltejs/kit") {
		result.Framework = "sveltekit"
		result.Type = "regular"
		result.BuildTool = detectBuildToolFromFiles(dir, "sveltekit")
		result.Detected = true
		return result
	}

	// -- 7. nuxt.config.[ts|js] — before vite.config (Nuxt uses Vite internally).
	if fileExistsAny(dir, "nuxt.config.ts", "nuxt.config.js") {
		result.Framework = "nuxt"
		result.Type = "regular"
		result.Detected = true
		return result
	}

	// -- 8. Vite.config.[ts|js] + package.json deps --.
	if fileExistsAny(dir, "vite.config.ts", "vite.config.js") {
		result.Type = "spa"
		result.BuildTool = "vite"
		result.Detected = true
		switch {
		case hasDep(earlyDeps, "react"):
			result.Framework = "react"
		case hasDep(earlyDeps, "vue"):
			result.Framework = "vue"
		case hasDep(earlyDeps, "svelte"):
			result.Framework = "svelte"
		default:
			result.Framework = "vanilla-javascript"
		}
		return result
	}

	// -- 9. Next.config.[js|ts|mjs] --.
	if fileExistsAny(dir, "next.config.js", "next.config.ts", "next.config.mjs") {
		result.Framework = "nextjs"
		result.Type = "regular"
		result.Detected = true
		return result
	}

	// -- 10. svelte.config.[js|ts] — only label as sveltekit when @sveltejs/kit dep is confirmed.
	if fileExistsAny(dir, "svelte.config.js", "svelte.config.ts") {
		framework := "sveltekit"
		appType := "regular"
		if len(earlyDeps) > 0 && !hasDep(earlyDeps, "@sveltejs/kit") {
			framework = "svelte"
			appType = "spa"
		}
		result.Framework = framework
		result.Type = appType
		result.BuildTool = detectBuildToolFromFiles(dir, framework)
		result.Detected = true
		return result
	}

	// Create-expo-app has generated app.json (not expo.json) since SDK 46 (2022).
	// Check app.json first; fall back to expo.json for legacy projects.
	if isExpoProject(dir) || fileExists(dir, "expo.json") {
		result.Framework = "expo"
		result.Type = "native"
		result.Detected = true
		return result
	}

	// -- 12. .csproj --.
	if content, ok := findCsprojContent(dir); ok {
		if fw, qsType, found := detectFromCsproj(content); found {
			result.Framework = fw
			result.Type = qsType
			if fw == "maui" || fw == "dotnet-mobile" {
				result.BundleID = readDotnetMobileBundleID(content)
			}
			result.Detected = true
			return result
		}
	}

	// -- 13. Android (native Java/Kotlin) — before Java build files to avoid vanilla-java misdetection.
	// Excludes React Native projects which also have AndroidManifest.xml in a sub-project.
	if fileExists(dir, filepath.Join("app", "src", "main", "AndroidManifest.xml")) && !hasDep(earlyDeps, "react-native") {
		result.Framework = "android"
		result.Type = "native"
		result.BuildTool = "gradle"
		result.BundleID = readAndroidApplicationID(dir)
		result.Detected = true
		return result
	}

	// -- 14. iOS Swift — .xcodeproj or Package.swift (excludes Vapor server-side Swift).
	if hasXcodeprojDir(dir) || (fileExists(dir, "Package.swift") && !isVaporSwiftPackage(dir)) {
		result.Framework = "ios-swift"
		result.Type = "native"
		result.BundleID = readIOSBundleID(dir)
		result.Detected = true
		return result
	}

	// -- 15. Pom.xml / build.gradle (Java) --.
	if content, buildTool, ok := findJavaBuildContent(dir); ok {
		result.Framework = detectJavaFramework(content)
		result.Type = "regular"
		result.BuildTool = buildTool
		result.Detected = true
		return result
	}

	// -- 16. Go.mod --.
	if fileExists(dir, "go.mod") {
		result.Framework = "vanilla-go"
		result.Type = "regular"
		result.Detected = true
		return result
	}

	// -- 17. Gemfile (Ruby on Rails) --.
	if data, ok := readFileContent(dir, "Gemfile"); ok {
		if strings.Contains(data, "rails") {
			result.Framework = "rails"
			result.Type = "regular"
			result.Detected = true
			return result
		}
	}

	// -- 18. Python dependency files (requirements.txt, pyproject.toml, Pipfile) --.
	for _, pyFile := range []string{"requirements.txt", "pyproject.toml", "Pipfile", "Pipfile.lock"} {
		if data, ok := readFileContent(dir, pyFile); ok {
			lower := strings.ToLower(data)
			if strings.Contains(lower, "flask") {
				result.Framework = "vanilla-python"
				result.Type = "regular"
				result.Detected = true
				return result
			}
			if strings.Contains(lower, "django") {
				result.Framework = "django"
				result.Type = "regular"
				result.Detected = true
				return result
			}
		}
	}

	// -- 19. Package.json dep scanning (lowest priority) --
	// Note: Ionic deps are already handled above (step 2).
	if len(earlyDeps) > 0 {
		candidates := collectPackageJSONCandidates(earlyDeps)
		switch len(candidates) {
		case 1:
			c := candidates[0]
			result.Framework = c.framework
			result.Type = c.qsType
			result.BuildTool = c.buildTool
			result.Detected = true
			// React Native uses the same android/app/build.gradle structure as Flutter.
			if c.framework == "react-native" {
				result.BundleID = readMobileBundleID(dir)
			}
		default:
			if len(candidates) > 1 {
				result.Type = "regular" // All package.json web deps are regular/native.
				result.Detected = true
				for _, c := range candidates {
					result.AmbiguousFrameworks = append(result.AmbiguousFrameworks, c.framework)
				}
			}
		}
	}

	return result
}

// collectPackageJSONCandidates returns framework candidates from package.json deps.
// Ionic deps are handled earlier in DetectProject (step 2) and excluded here.
func collectPackageJSONCandidates(deps map[string]bool) []detectionCandidate {
	var candidates []detectionCandidate
	// React-native without expo (expo check would have matched earlier in DetectProject).
	if hasDep(deps, "react-native") {
		candidates = append(candidates, detectionCandidate{framework: "react-native", qsType: "native"})
	}
	if hasDep(deps, "express") {
		candidates = append(candidates, detectionCandidate{framework: "express", qsType: "regular"})
	}
	if hasDep(deps, "hono") {
		candidates = append(candidates, detectionCandidate{framework: "hono", qsType: "regular"})
	}
	if hasDep(deps, "fastify") {
		candidates = append(candidates, detectionCandidate{framework: "fastify", qsType: "regular"})
	}
	return candidates
}

// detectFromCsproj returns framework and type from .csproj file content.
func detectFromCsproj(content string) (framework, qsType string, found bool) {
	switch {
	case strings.Contains(content, "Microsoft.AspNetCore.Components"):
		return "aspnet-blazor", "regular", true
	case strings.Contains(content, "Microsoft.Owin"):
		return "aspnet-owin", "regular", true
	case strings.Contains(content, "Microsoft.AspNetCore.Mvc"):
		return "aspnet-mvc", "regular", true
	// .NET 6+: MVC is built-in via Sdk.Web — checked after Blazor/OWIN to avoid false positives.
	case strings.Contains(content, `Sdk="Microsoft.NET.Sdk.Web"`):
		return "aspnet-mvc", "regular", true
	case strings.Contains(content, "Microsoft.Maui"):
		return "maui", "native", true
	// Mobile TFM (net*-android or net*-ios) without MAUI → dotnet-mobile.
	case mobileTFMRegex.MatchString(content):
		return "dotnet-mobile", "native", true
	case strings.Contains(content, "-windows"):
		return "wpf-winforms", "native", true
	}
	return "", "", false
}

// detectJavaFramework returns the framework key from Java build file content.
func detectJavaFramework(content string) string {
	lower := strings.ToLower(content)
	switch {
	case strings.Contains(lower, "spring-boot") ||
		strings.Contains(lower, "springframework.boot"):
		return "spring-boot"
	case strings.Contains(lower, "javax.ee") ||
		strings.Contains(lower, "jakarta.ee") ||
		strings.Contains(lower, "javax.servlet") ||
		strings.Contains(lower, "jakarta.servlet") ||
		// Jakarta.platform:jakarta.jakartaee-api is the standard BOM for Jakarta EE 9+.
		strings.Contains(lower, "jakarta.platform"):
		return "java-ee"
	default:
		return "vanilla-java"
	}
}

func isExpoProject(dir string) bool {
	data, err := os.ReadFile(filepath.Join(dir, "app.json"))
	if err != nil {
		return false
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(data, &obj); err != nil {
		return false
	}
	_, hasExpoKey := obj["expo"]
	return hasExpoKey
}

// readRawExpoScheme reads the "expo.scheme" field from app.json without validating it.
// Returns the raw string value (may be empty or invalid).
func readRawExpoScheme(dir string) string {
	data, err := os.ReadFile(filepath.Join(dir, "app.json"))
	if err != nil {
		return ""
	}
	var obj struct {
		Expo struct {
			Scheme string `json:"scheme"`
		} `json:"expo"`
	}
	if err := json.Unmarshal(data, &obj); err != nil {
		return ""
	}
	return obj.Expo.Scheme
}

// readExpoScheme reads the "expo.scheme" from app.json and validates it per RFC 3986.
// Returns empty string if absent, invalid, or on any error.
func readExpoScheme(dir string) string {
	scheme := readRawExpoScheme(dir)
	if !isValidURIScheme(scheme) {
		return ""
	}
	return scheme
}

// scheme = ALPHA *( ALPHA / DIGIT / "+" / "-" / "." ).
func isValidURIScheme(s string) bool {
	if len(s) == 0 {
		return false
	}
	for i, r := range s {
		if i == 0 {
			if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')) {
				return false
			}
		} else {
			if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
				(r >= '0' && r <= '9') || r == '+' || r == '-' || r == '.') {
				return false
			}
		}
	}
	return true
}

// pubspecHasFlutterWebTarget returns true if "web:" is nested under "flutter:" in pubspec.yaml.
func pubspecHasFlutterWebTarget(data string) bool {
	inFlutterSection := false
	for _, line := range strings.Split(data, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		// A line with no leading whitespace is a top-level YAML key.
		isTopLevel := len(line) > 0 && line[0] != ' ' && line[0] != '\t'
		if isTopLevel {
			inFlutterSection = strings.HasPrefix(line, "flutter:")
			continue
		}
		if inFlutterSection && strings.HasPrefix(trimmed, "web:") {
			return true
		}
	}
	return false
}

func dirExists(dir, name string) bool {
	info, err := os.Stat(filepath.Join(dir, name))
	return err == nil && info.IsDir()
}

func fileExists(dir, name string) bool {
	_, err := os.Stat(filepath.Join(dir, name))
	return err == nil
}

func fileExistsAny(dir string, names ...string) bool {
	for _, name := range names {
		if fileExists(dir, name) {
			return true
		}
	}
	return false
}

const maxDetectionFileSize = 10 * 1024 * 1024 // 10 MB.

func readFileContent(dir, name string) (string, bool) {
	filePath := filepath.Join(dir, name)
	info, err := os.Stat(filePath)
	if err != nil || info.Size() > maxDetectionFileSize {
		return "", false
	}
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", false
	}
	return string(data), true
}

// readPackageJSONDeps reads package.json and returns a set of all dependency names
// (from both "dependencies" and "devDependencies").
func readPackageJSONDeps(dir string) map[string]bool {
	data, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
		return make(map[string]bool)
	}
	var pkg struct {
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return make(map[string]bool)
	}
	deps := make(map[string]bool)
	for k := range pkg.Dependencies {
		deps[k] = true
	}
	for k := range pkg.DevDependencies {
		deps[k] = true
	}
	return deps
}

func hasDep(deps map[string]bool, name string) bool {
	return deps[name]
}

func findCsprojContent(dir string) (string, bool) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", false
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".csproj") {
			if data, fileErr := os.ReadFile(filepath.Join(dir, e.Name())); fileErr == nil {
				return string(data), true
			}
		}
	}
	return "", false
}

// mobileTFMRegex matches .NET mobile Target Framework Monikers (net*-android or net*-ios).
// A bare substring match on "-android" / "-ios" can produce false positives on package
// names such as "Newtonsoft.Json-ios" or condition attributes. Requiring the leading
// net<major>.<minor> prefix eliminates those false positives.
var mobileTFMRegex = regexp.MustCompile(`net\d+\.\d+-(?:android|ios)`)

// detectBuildToolFromFiles detects the build tool by checking for config files in dir.
// Falls back to the conventional default for the framework if no relevant file is found.
func detectBuildToolFromFiles(dir, framework string) string {
	if fileExistsAny(dir, "vite.config.ts", "vite.config.js") {
		return "vite"
	}
	if fileExists(dir, "pom.xml") {
		return "maven"
	}
	if fileExistsAny(dir, "build.gradle", "build.gradle.kts") {
		return "gradle"
	}
	if fileExists(dir, "composer.json") {
		return "composer"
	}
	// Framework-specific defaults as fallback.
	switch framework {
	case "ionic-react", "ionic-vue", "sveltekit":
		return "vite"
	case "spring-boot", "vanilla-java", "java-ee":
		return "maven"
	case "laravel", "vanilla-php":
		return "composer"
	}
	return ""
}

// findJavaBuildContent finds pom.xml or build.gradle and returns content + build tool name.
func findJavaBuildContent(dir string) (content, buildTool string, ok bool) {
	if data, err := os.ReadFile(filepath.Join(dir, "pom.xml")); err == nil {
		return string(data), "maven", true
	}
	if data, err := os.ReadFile(filepath.Join(dir, "build.gradle")); err == nil {
		return string(data), "gradle", true
	}
	if data, err := os.ReadFile(filepath.Join(dir, "build.gradle.kts")); err == nil {
		return string(data), "gradle", true
	}
	return "", "", false
}

// readMobileBundleID reads the application/bundle ID for Flutter and React Native projects.
// Checks android/app/build.gradle first, then falls back to iOS project files.
func readMobileBundleID(dir string) string {
	if data, err := os.ReadFile(filepath.Join(dir, "android", "app", "build.gradle")); err == nil {
		if id := extractGradleApplicationID(string(data)); id != "" {
			return id
		}
	}
	// IOS fallback: try project.pbxproj first (the canonical source for the bundle ID),
	// then Info.plist (only when CFBundleIdentifier is not a build variable reference).
	return readIOSBundleID(dir)
}

// readIOSBundleID reads the bundle ID from iOS project files (pbxproj then Info.plist).
func readIOSBundleID(dir string) string {
	// Flutter path.
	if data, err := os.ReadFile(filepath.Join(dir, "ios", "Runner.xcodeproj", "project.pbxproj")); err == nil {
		if id := extractPbxprojBundleID(string(data)); id != "" {
			return id
		}
	}
	// Native Xcode projects place the .xcodeproj at the root of the repo.
	if entries, err := os.ReadDir(dir); err == nil {
		for _, e := range entries {
			if e.IsDir() && strings.HasSuffix(e.Name(), ".xcodeproj") {
				pbx := filepath.Join(dir, e.Name(), "project.pbxproj")
				if data, err := os.ReadFile(pbx); err == nil {
					if id := extractPbxprojBundleID(string(data)); id != "" {
						return id
					}
				}
			}
		}
	}
	if data, err := os.ReadFile(filepath.Join(dir, "ios", "Runner", "Info.plist")); err == nil {
		if id := extractInfoPlistBundleID(string(data)); id != "" {
			return id
		}
	}
	return ""
}

// pbxprojBundleIDRegex matches PRODUCT_BUNDLE_IDENTIFIER = com.example.app; in project.pbxproj.
var pbxprojBundleIDRegex = regexp.MustCompile(`PRODUCT_BUNDLE_IDENTIFIER\s*=\s*([a-zA-Z][a-zA-Z0-9._-]*)\s*;`)

// extractPbxprojBundleID extracts PRODUCT_BUNDLE_IDENTIFIER from project.pbxproj,
// skipping test-target bundle IDs (those ending in "Tests").
func extractPbxprojBundleID(content string) string {
	all := pbxprojBundleIDRegex.FindAllStringSubmatch(content, -1)
	for _, m := range all {
		if len(m) < 2 {
			continue
		}
		id := strings.TrimSpace(m[1])
		// Skip test-target bundle IDs. In Xcode-generated project.pbxproj files,
		// test targets appear before the app target. Both dotted suffixes
		// (com.example.app.Tests, com.example.app.UITests) and concatenated
		// suffixes (com.example.appTests) all end in "Tests".
		if strings.HasSuffix(id, "Tests") {
			continue
		}
		return id
	}
	return ""
}

// infoPlistBundleIDRegex matches <key>CFBundleIdentifier</key> followed by <string>value</string>.
var infoPlistBundleIDRegex = regexp.MustCompile(`<key>CFBundleIdentifier</key>\s*<string>([^<]+)</string>`)

// extractInfoPlistBundleID extracts CFBundleIdentifier from Info.plist content.
// Returns empty string if absent or if the value is a build variable reference (e.g. "$(..)").
func extractInfoPlistBundleID(content string) string {
	matches := infoPlistBundleIDRegex.FindStringSubmatch(content)
	if len(matches) < 2 {
		return ""
	}
	id := strings.TrimSpace(matches[1])
	// Skip Xcode variable references such as "$(PRODUCT_BUNDLE_IDENTIFIER)".
	if strings.Contains(id, "$") {
		return ""
	}
	return id
}

// gradleAppIDRegex matches applicationId in build.gradle (Groovy and Kotlin DSL forms).
var gradleAppIDRegex = regexp.MustCompile(`applicationId\s*=?\s*["']([a-zA-Z][a-zA-Z0-9._-]*)["']`)

// extractGradleApplicationID extracts the applicationId value from build.gradle content.
func extractGradleApplicationID(content string) string {
	matches := gradleAppIDRegex.FindStringSubmatch(content)
	if len(matches) < 2 {
		return ""
	}
	return matches[1]
}

// capacitorTSAppIDRegex extracts appId from capacitor.config.ts (single or double-quoted).
var capacitorTSAppIDRegex = regexp.MustCompile(`appId\s*:\s*(?:'([^']*)'|"([^"]*)")`)

// readCapacitorAppID reads the "appId" field from capacitor.config.json or
// capacitor.config.ts in dir. Capacitor v3+ defaults to the TypeScript config.
// Returns empty string if neither file is present or the field is missing.
func readCapacitorAppID(dir string) string {
	// Try JSON config first (Capacitor v2 and v3+ both support it).
	if data, err := os.ReadFile(filepath.Join(dir, "capacitor.config.json")); err == nil {
		var cfg struct {
			AppID string `json:"appId"`
		}
		if jsonErr := json.Unmarshal(data, &cfg); jsonErr == nil && cfg.AppID != "" {
			return cfg.AppID
		}
	}
	// Fall back to TypeScript config (Capacitor v3+ default).
	// Process line-by-line to skip comment lines that may contain an appId value
	// (e.g. "// appId: 'old.value'") which would otherwise be matched first.
	if data, err := os.ReadFile(filepath.Join(dir, "capacitor.config.ts")); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "//") {
				continue
			}
			if m := capacitorTSAppIDRegex.FindStringSubmatch(line); len(m) >= 3 {
				// M[1] = single-quoted match, m[2] = double-quoted match.
				if m[1] != "" {
					return m[1]
				}
				if m[2] != "" {
					return m[2]
				}
			}
		}
	}
	return ""
}

// readDotnetMobileBundleID extracts the <ApplicationId> element from .csproj content.
// Used for MAUI and .NET Mobile apps to generate callback URL guidance.
// Returns empty string if the element is absent.
func readDotnetMobileBundleID(content string) string {
	matches := csprojAppIDRegex.FindStringSubmatch(content)
	if len(matches) < 2 {
		return ""
	}
	return strings.TrimSpace(matches[1])
}

// csprojAppIDRegex matches the <ApplicationId> element in a .csproj file.
var csprojAppIDRegex = regexp.MustCompile(`<ApplicationId>\s*([a-zA-Z][a-zA-Z0-9._-]*)\s*</ApplicationId>`)

// isVaporSwiftPackage returns true if Package.swift in dir contains a Vapor dependency.
// Vapor's Package.swift references "vapor/vapor.git" or the package URL contains "vapor/vapor".
func isVaporSwiftPackage(dir string) bool {
	data, ok := readFileContent(dir, "Package.swift")
	if !ok {
		return false
	}
	return strings.Contains(data, "vapor/vapor")
}

// hasXcodeprojDir returns true if any directory entry in dir ends with ".xcodeproj".
// An .xcodeproj bundle is the primary signal file for Xcode-based iOS/macOS Swift projects.
func hasXcodeprojDir(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if e.IsDir() && strings.HasSuffix(e.Name(), ".xcodeproj") {
			return true
		}
	}
	return false
}

// readAndroidApplicationID reads the applicationId from app/build.gradle or app/build.gradle.kts.
func readAndroidApplicationID(dir string) string {
	for _, name := range []string{
		filepath.Join("app", "build.gradle"),
		filepath.Join("app", "build.gradle.kts"),
	} {
		if data, err := os.ReadFile(filepath.Join(dir, name)); err == nil {
			if id := extractGradleApplicationID(string(data)); id != "" {
				return id
			}
		}
	}
	return ""
}

// detectionFriendlyAppType returns a concise label for the detection summary display.
func detectionFriendlyAppType(qsType string) string {
	switch qsType {
	case "spa":
		return "Single Page App"
	case "regular":
		return "Regular Web App"
	case "native":
		return "Native / Mobile"
	case "m2m":
		return "Machine to Machine"
	default:
		return qsType
	}
}
