package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// DetectionResult holds the values resolved by scanning the working directory.
// Fields are empty/zero when not detected. AmbiguousCandidates is populated when
// multiple package.json deps match and the framework cannot be determined uniquely.
type DetectionResult struct {
	Framework           string
	Type                string   // "spa" | "regular" | "native"
	BuildTool           string   // "vite" | "maven" | "gradle" | "composer" | "" (NA)
	Port                int      // 0 means no applicable default
	AppName             string   // basename of the working directory
	Detected            bool     // true if any signal file matched
	AmbiguousCandidates []string // set when >1 package.json dep matched
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

	// ── 1. angular.json ─────────────────────────────────────────────────────
	if fileExists(dir, "angular.json") {
		result.Framework = "angular"
		result.Type = "spa"
		result.Port = 4200
		result.Detected = true
		return result
	}

	// ── 2. pubspec.yaml (Flutter) ────────────────────────────────────────────
	if data, ok := readFileContent(dir, "pubspec.yaml"); ok {
		if strings.Contains(data, "sdk: flutter") {
			result.Detected = true
			if isFlutterWeb(dir) {
				result.Framework = "flutter-web"
				result.Type = "spa"
			} else {
				result.Framework = "flutter"
				result.Type = "native"
			}
			return result
		}
	}

	// ── 3. vite.config.[ts|js] + package.json deps ───────────────────────────
	if fileExistsAny(dir, "vite.config.ts", "vite.config.js") {
		deps := readPackageJSONDeps(dir)
		result.Type = "spa"
		result.BuildTool = "vite"
		result.Port = 5173
		result.Detected = true
		switch {
		case hasDep(deps, "react"):
			result.Framework = "react"
		case hasDep(deps, "vue"):
			result.Framework = "vue"
		case hasDep(deps, "svelte"):
			result.Framework = "svelte"
		default:
			result.Framework = "vanilla-javascript"
		}
		return result
	}

	// ── 4. next.config.[js|ts|mjs] ──────────────────────────────────────────
	if fileExistsAny(dir, "next.config.js", "next.config.ts", "next.config.mjs") {
		result.Framework = "nextjs"
		result.Type = "regular"
		result.Port = 3000
		result.Detected = true
		return result
	}

	// ── 5. nuxt.config.[ts|js] ───────────────────────────────────────────────
	if fileExistsAny(dir, "nuxt.config.ts", "nuxt.config.js") {
		result.Framework = "nuxt"
		result.Type = "regular"
		result.Port = 3000
		result.Detected = true
		return result
	}

	// ── 6. svelte.config.[js|ts] ─────────────────────────────────────────────
	if fileExistsAny(dir, "svelte.config.js", "svelte.config.ts") {
		result.Framework = "sveltekit"
		result.Type = "regular"
		result.Detected = true
		return result
	}

	// ── 7. expo.json ─────────────────────────────────────────────────────────
	if fileExists(dir, "expo.json") {
		result.Framework = "expo"
		result.Type = "native"
		result.Detected = true
		return result
	}

	// ── 8. .csproj ───────────────────────────────────────────────────────────
	if content, ok := findCsprojContent(dir); ok {
		if fw, qsType, found := detectFromCsproj(content); found {
			result.Framework = fw
			result.Type = qsType
			result.Detected = true
			return result
		}
	}

	// ── 9. pom.xml / build.gradle (Java) ─────────────────────────────────────
	if content, buildTool, ok := findJavaBuildContent(dir); ok {
		fw, port := detectJavaFramework(content)
		result.Framework = fw
		result.Type = "regular"
		result.BuildTool = buildTool
		result.Port = port
		result.Detected = true
		return result
	}

	// ── 10. composer.json (PHP) ───────────────────────────────────────────────
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

	// ── 11. go.mod ───────────────────────────────────────────────────────────
	if fileExists(dir, "go.mod") {
		result.Framework = "vanilla-go"
		result.Type = "regular"
		result.Detected = true
		return result
	}

	// ── 12. Gemfile (Ruby on Rails) ──────────────────────────────────────────
	if data, ok := readFileContent(dir, "Gemfile"); ok {
		if strings.Contains(data, "rails") {
			result.Framework = "rails"
			result.Type = "regular"
			result.Port = 3000
			result.Detected = true
			return result
		}
	}

	// ── 13. requirements.txt / pyproject.toml (Python / Flask) ───────────────
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

	// ── 14. package.json dep scanning (lowest priority) ──────────────────────
	deps := readPackageJSONDeps(dir)
	if len(deps) > 0 {
		candidates := collectPackageJSONCandidates(deps)
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
				result.Type = "regular" // all package.json web deps are regular/native
				result.Detected = true
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
	// react-native without expo (expo.json would have matched earlier)
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
	case strings.Contains(content, "Microsoft.AspNetCore.Mvc"):
		return "aspnet-mvc", "regular", true
	case strings.Contains(content, "Microsoft.Owin"):
		return "aspnet-owin", "regular", true
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
		strings.Contains(lower, "jakarta.servlet"):
		return "java-ee", 0
	default:
		return "vanilla-java", 0
	}
}

// isFlutterWeb returns true if the project has web platform support enabled.
// It checks for the standard web/ directory that Flutter creates for web targets.
func isFlutterWeb(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, "web", "index.html"))
	return err == nil
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
