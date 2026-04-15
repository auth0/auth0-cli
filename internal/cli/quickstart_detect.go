package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// DetectionResult holds the values resolved by scanning the working directory.
// Fields are empty/zero when not detected. AmbiguousCandidates is populated when
// multiple package.json deps match and the framework cannot be determined uniquely.
type DetectionResult struct {
	Framework           string
	Type                string   // "spa" | "regular" | "native".
	BuildTool           string   // "vite" | "maven" | "gradle" | "composer" | "" (NA).
	Port                int      // 0 means no applicable default.
	AppName             string   // Basename of the working directory.
	Detected            bool     // True if any signal file matched.
	AmbiguousCandidates []string // Set when >1 package.json dep matched.
}

// detectionCandidate is used internally during package.json dep scanning.
type detectionCandidate struct {
	framework string
	qsType    string
	buildTool string
	port      int
}

// DetectProject scans dir for framework signal files and returns a DetectionResult.
// Rules follow the priority order from the spec: config files beat package.json scanning.
func DetectProject(dir string) DetectionResult {
	result := DetectionResult{
		AppName: filepath.Base(dir),
	}
	if name := readProjectName(dir); name != "" {
		result.AppName = name
	}

	// Read package.json deps early — needed for checks that must precede file-based signals.
	earlyDeps := readPackageJSONDeps(dir)

	// ── 1. Ionic (package.json deps — must check BEFORE angular.json and vite.config) ──.
	if hasDep(earlyDeps, "@ionic/angular") {
		result.Framework = "ionic-angular"
		result.Type = "native"
		result.Detected = true
		return result
	}
	if hasDep(earlyDeps, "@ionic/react") {
		result.Framework = "ionic-react"
		result.Type = "native"
		result.BuildTool = detectBuildToolFromFiles(dir, "ionic-react")
		result.Detected = true
		return result
	}
	if hasDep(earlyDeps, "@ionic/vue") {
		result.Framework = "ionic-vue"
		result.Type = "native"
		result.BuildTool = detectBuildToolFromFiles(dir, "ionic-vue")
		result.Detected = true
		return result
	}

	// ── 2. Angular.json ────────────────────────────────────────────────────.
	if fileExists(dir, "angular.json") {
		result.Framework = "angular"
		result.Type = "spa"
		result.Port = detectPortFromConfig(dir, "angular", 4200)
		result.Detected = true
		return result
	}

	// ── 3. Pubspec.yaml (Flutter) ───────────────────────────────────────────.
	if data, ok := readFileContent(dir, "pubspec.yaml"); ok {
		if strings.Contains(data, "sdk: flutter") {
			result.Detected = true
			// Flutter create (default) has included web/ since Flutter 2.10, so web/ alone
			// is not a reliable signal for web-only intent.
			if dirExists(dir, "android") || dirExists(dir, "ios") {
				result.Framework = "flutter"
				result.Type = "native"
			} else {
				result.Framework = "flutter-web"
				result.Type = "spa"
			}
			return result
		}
	}

	// ── 4. Composer.json (PHP) — BEFORE vite.config to prevent Laravel misdetection ──.
	// Laravel 10+ ships with vite.config.js; checking composer.json first avoids a
	// false-positive Vanilla-JavaScript match for Laravel projects.
	if data, ok := readFileContent(dir, "composer.json"); ok {
		result.BuildTool = "composer"
		result.Type = "regular"
		result.Detected = true
		if strings.Contains(data, "laravel/framework") {
			result.Framework = "laravel"
			result.Port = 8000
		} else {
			result.Framework = "vanilla-php"
		}
		return result
	}

	// ── 5. SvelteKit (@sveltejs/kit dep — BEFORE vite.config) ───────────────.
	// Plain Svelte+Vite also creates svelte.config.js and vite.config.ts, so
	// @sveltejs/kit in package.json is the only reliable distinguishing signal.
	if hasDep(earlyDeps, "@sveltejs/kit") {
		result.Framework = "sveltekit"
		result.Type = "regular"
		result.BuildTool = detectBuildToolFromFiles(dir, "sveltekit")
		result.Port = detectPortFromConfig(dir, "sveltekit", 3000)
		result.Detected = true
		return result
	}

	// ── 6. Vite.config.[ts|js] + package.json deps ──────────────────────────.
	if fileExistsAny(dir, "vite.config.ts", "vite.config.js") {
		result.Type = "spa"
		result.BuildTool = "vite"
		result.Port = detectPortFromConfig(dir, "vite", 5173)
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

	// ── 7. Next.config.[js|ts|mjs] ─────────────────────────────────────────.
	if fileExistsAny(dir, "next.config.js", "next.config.ts", "next.config.mjs") {
		result.Framework = "nextjs"
		result.Type = "regular"
		result.Port = detectPortFromConfig(dir, "nextjs", 3000)
		result.Detected = true
		return result
	}

	// ── 8. Nuxt.config.[ts|js] ──────────────────────────────────────────────.
	if fileExistsAny(dir, "nuxt.config.ts", "nuxt.config.js") {
		result.Framework = "nuxt"
		result.Type = "regular"
		result.Port = 3000
		result.Detected = true
		return result
	}

	// ── 9. Svelte.config.[js|ts] ────────────────────────────────────────────.
	if fileExistsAny(dir, "svelte.config.js", "svelte.config.ts") {
		result.Framework = "sveltekit"
		result.Type = "regular"
		result.BuildTool = detectBuildToolFromFiles(dir, "sveltekit")
		result.Port = detectPortFromConfig(dir, "sveltekit", 3000)
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

	// ── 11. .csproj ──────────────────────────────────────────────────────────.
	if content, ok := findCsprojContent(dir); ok {
		if fw, qsType, found := detectFromCsproj(content); found {
			result.Framework = fw
			result.Type = qsType
			result.Detected = true
			return result
		}
	}

	// ── 12. Pom.xml / build.gradle (Java) ────────────────────────────────────.
	if content, buildTool, ok := findJavaBuildContent(dir); ok {
		fw, port := detectJavaFramework(content)
		result.Framework = fw
		result.Type = "regular"
		result.BuildTool = buildTool
		result.Port = port
		result.Detected = true
		return result
	}

	// ── 13. Go.mod ──────────────────────────────────────────────────────────.
	if fileExists(dir, "go.mod") {
		result.Framework = "vanilla-go"
		result.Type = "regular"
		result.Detected = true
		return result
	}

	// ── 14. Gemfile (Ruby on Rails) ─────────────────────────────────────────.
	if data, ok := readFileContent(dir, "Gemfile"); ok {
		if strings.Contains(data, "rails") {
			result.Framework = "rails"
			result.Type = "regular"
			result.Port = 3000
			result.Detected = true
			return result
		}
	}

	// ── 15. Requirements.txt / pyproject.toml (Python / Flask) ──────────────.
	for _, pyFile := range []string{"requirements.txt", "pyproject.toml"} {
		if data, ok := readFileContent(dir, pyFile); ok {
			if strings.Contains(strings.ToLower(data), "flask") {
				result.Framework = "vanilla-python"
				result.Type = "regular"
				result.Port = 5000
				result.Detected = true
				return result
			}
		}
	}

	// ── 16. Package.json dep scanning (lowest priority) ─────────────────────.
	// Note: Ionic deps are already handled above (step 1).
	if len(earlyDeps) > 0 {
		candidates := collectPackageJSONCandidates(earlyDeps)
		switch len(candidates) {
		case 1:
			c := candidates[0]
			result.Framework = c.framework
			result.Type = c.qsType
			result.BuildTool = c.buildTool
			result.Port = c.port
			result.Detected = true
		default:
			if len(candidates) > 1 {
				result.Type = "regular" // All package.json web deps are regular/native.
				result.Detected = true
				// Use the common port if all candidates agree (e.g. express + hono both use 3000).
				commonPort := candidates[0].port
				for _, c := range candidates {
					if c.port != commonPort {
						commonPort = 0
						break
					}
				}
				result.Port = commonPort
				for _, c := range candidates {
					result.AmbiguousCandidates = append(result.AmbiguousCandidates, c.framework)
				}
			}
		}
	}

	return result
}

// collectPackageJSONCandidates returns all framework candidates found in deps.
func collectPackageJSONCandidates(deps map[string]bool) []detectionCandidate {
	var candidates []detectionCandidate
	if hasDep(deps, "@ionic/angular") {
		candidates = append(candidates, detectionCandidate{framework: "ionic-angular", qsType: "native"})
	}
	if hasDep(deps, "@ionic/react") {
		candidates = append(candidates, detectionCandidate{framework: "ionic-react", qsType: "native", buildTool: "vite"})
	}
	if hasDep(deps, "@ionic/vue") {
		candidates = append(candidates, detectionCandidate{framework: "ionic-vue", qsType: "native", buildTool: "vite"})
	}
	// React-native without expo (expo check would have matched earlier in DetectProject).
	if hasDep(deps, "react-native") {
		candidates = append(candidates, detectionCandidate{framework: "react-native", qsType: "native"})
	}
	if hasDep(deps, "express") {
		candidates = append(candidates, detectionCandidate{framework: "express", qsType: "regular", port: 3000})
	}
	if hasDep(deps, "hono") {
		candidates = append(candidates, detectionCandidate{framework: "hono", qsType: "regular", port: 3000})
	}
	if hasDep(deps, "fastify") {
		candidates = append(candidates, detectionCandidate{framework: "fastify", qsType: "regular", port: 3000})
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
	// .NET 6+: MVC is built-in via Microsoft.NET.Sdk.Web — no PackageReference generated.
	// Check this after Blazor (AspNetCore.Components) and OWIN to avoid false positives.
	case strings.Contains(content, `Sdk="Microsoft.NET.Sdk.Web"`):
		return "aspnet-mvc", "regular", true
	case strings.Contains(content, "Microsoft.Maui") ||
		strings.Contains(content, "-android") ||
		strings.Contains(content, "-ios"):
		return "maui", "native", true
	case strings.Contains(content, "-windows"):
		return "wpf-winforms", "native", true
	}
	return "", "", false
}

// detectJavaFramework returns the framework key and default port from Java build file content.
func detectJavaFramework(content string) (framework string, port int) {
	lower := strings.ToLower(content)
	switch {
	case strings.Contains(lower, "spring-boot"):
		return "spring-boot", 8080
	case strings.Contains(lower, "javax.ee") ||
		strings.Contains(lower, "jakarta.ee") ||
		strings.Contains(lower, "javax.servlet") ||
		strings.Contains(lower, "jakarta.servlet") ||
		// Jakarta.platform:jakarta.jakartaee-api is the standard BOM for Jakarta EE 9+.
		strings.Contains(lower, "jakarta.platform"):
		return "java-ee", 0
	default:
		return "vanilla-java", 0
	}
}

// Create-expo-app has generated app.json (not expo.json) since SDK 46 in 2022.
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

// dirExists returns true if the named entry in dir is a directory.
func dirExists(dir, name string) bool {
	info, err := os.Stat(filepath.Join(dir, name))
	return err == nil && info.IsDir()
}

// fileExists returns true if the named file exists in dir.
func fileExists(dir, name string) bool {
	_, err := os.Stat(filepath.Join(dir, name))
	return err == nil
}

// fileExistsAny returns true if any of the named files exist in dir.
func fileExistsAny(dir string, names ...string) bool {
	for _, name := range names {
		if fileExists(dir, name) {
			return true
		}
	}
	return false
}

// readFileContent reads a file and returns its content as a string.
func readFileContent(dir, name string) (string, bool) {
	data, err := os.ReadFile(filepath.Join(dir, name))
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
		return nil
	}
	var pkg struct {
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil
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

// readPackageJSONName reads the "name" field from package.json in dir.
// Returns empty string if not found or on any error.
func readPackageJSONName(dir string) string {
	data, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
		return ""
	}
	var pkg struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return ""
	}
	return pkg.Name
}

// readProjectName tries to extract a meaningful project name from language-specific
// manifest files. It falls back to empty string if none are found; the caller then
// uses filepath.Base(dir).
func readProjectName(dir string) string {
	if name := readPackageJSONName(dir); name != "" {
		return name
	}
	if name := readGoModuleName(dir); name != "" {
		return name
	}
	if name := readPyprojectName(dir); name != "" {
		return name
	}
	if name := readPubspecName(dir); name != "" {
		return name
	}
	if name := readComposerName(dir); name != "" {
		return name
	}
	if name := readPomArtifactID(dir); name != "" {
		return name
	}
	return ""
}

// readGoModuleName reads the module path from go.mod and returns its last path segment.
func readGoModuleName(dir string) string {
	data, ok := readFileContent(dir, "go.mod")
	if !ok {
		return ""
	}
	for _, line := range strings.SplitN(data, "\n", 20) {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			modulePath := strings.TrimSpace(strings.TrimPrefix(line, "module "))
			return filepath.Base(modulePath)
		}
	}
	return ""
}

// readPyprojectName reads the project name from pyproject.toml ([project] or [tool.poetry] section).
func readPyprojectName(dir string) string {
	data, ok := readFileContent(dir, "pyproject.toml")
	if !ok {
		return ""
	}
	for _, line := range strings.Split(data, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "name ") && !strings.HasPrefix(line, "name=") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		val := strings.TrimSpace(parts[1])
		val = strings.Trim(val, `"'`)
		if val != "" {
			return val
		}
	}
	return ""
}

// readPubspecName reads the name field from pubspec.yaml.
func readPubspecName(dir string) string {
	data, ok := readFileContent(dir, "pubspec.yaml")
	if !ok {
		return ""
	}
	for _, line := range strings.Split(data, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "name:") {
			val := strings.TrimSpace(strings.TrimPrefix(trimmed, "name:"))
			if val != "" {
				return val
			}
		}
	}
	return ""
}

// readComposerName reads the package name from composer.json and returns the part after "/".
func readComposerName(dir string) string {
	data, err := os.ReadFile(filepath.Join(dir, "composer.json"))
	if err != nil {
		return ""
	}
	var pkg struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil || pkg.Name == "" {
		return ""
	}
	if idx := strings.LastIndex(pkg.Name, "/"); idx >= 0 {
		return pkg.Name[idx+1:]
	}
	return pkg.Name
}

// readPomArtifactID reads the first <artifactId> value from pom.xml.
func readPomArtifactID(dir string) string {
	data, ok := readFileContent(dir, "pom.xml")
	if !ok {
		return ""
	}
	const open = "<artifactId>"
	const closeTag = "</artifactId>"
	start := strings.Index(data, open)
	if start == -1 {
		return ""
	}
	start += len(open)
	end := strings.Index(data[start:], closeTag)
	if end == -1 {
		return ""
	}
	return strings.TrimSpace(data[start : start+end])
}

// hasDep returns true if the named dependency is in the deps set.
func hasDep(deps map[string]bool, name string) bool {
	return deps[name]
}

// findCsprojContent finds the first .csproj file in dir and returns its content.
func findCsprojContent(dir string) (string, bool) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", false
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".csproj") {
			if data, err := os.ReadFile(filepath.Join(dir, e.Name())); err == nil {
				return string(data), true
			}
		}
	}
	return "", false
}

// portPattern matches port assignments in config files, e.g. `port: 3001` or `"port": 3001`.
var portPattern = regexp.MustCompile(`"?port"?\s*:\s*(\d{4,5})`)

// extractPortFromContent returns the first port number found in content, or 0 if none found.
func extractPortFromContent(content string) int {
	matches := portPattern.FindStringSubmatch(content)
	if len(matches) < 2 {
		return 0
	}
	p, err := strconv.Atoi(matches[1])
	if err != nil || p < 1024 || p > 65535 {
		return 0
	}
	return p
}

// detectPortFromConfig tries to read the port from a project config file.
// It checks framework-specific files (vite.config.ts/js for vite-based projects,
// angular.json for Angular, next.config.* for Next.js). Falls back to defaultPort.
func detectPortFromConfig(dir, hint string, defaultPort int) int {
	switch hint {
	case "angular":
		if data, ok := readFileContent(dir, "angular.json"); ok {
			if p := extractPortFromContent(data); p > 0 {
				return p
			}
		}
	case "nextjs":
		for _, name := range []string{"next.config.ts", "next.config.js", "next.config.mjs"} {
			if data, ok := readFileContent(dir, name); ok {
				if p := extractPortFromContent(data); p > 0 {
					return p
				}
			}
		}
	case "django", "rails", "vanilla-go", "vanilla-python", "aspnet-mvc", "aspnet-blazor",
		"aspnet-owin", "vanilla-php", "vanilla-java", "java-ee", "spring-boot", "laravel",
		"express", "hono", "fastify", "nuxt":
		// Backend-only or non-vite frameworks: no config file to inspect, use default directly.
	default:
		// For vite-based projects (react, vue, svelte, sveltekit, ionic-*, etc.)
		for _, name := range []string{"vite.config.ts", "vite.config.js"} {
			if data, ok := readFileContent(dir, name); ok {
				if p := extractPortFromContent(data); p > 0 {
					return p
				}
			}
		}
	}
	return defaultPort
}

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
