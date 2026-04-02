package cli

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/auth0/go-auth0/management"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/auth0/auth0-cli/internal/auth0"
)

// ── test helpers ──────────────────────────────────────────────────────────────

func writeTestFile(t *testing.T, dir, name, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0600))
}

func mkTestDir(t *testing.T, dir, sub string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, sub), 0755))
}

// ── DetectProject – no signal ─────────────────────────────────────────────────

func TestDetectProject_NoDetection(t *testing.T) {
	dir := t.TempDir()
	got := DetectProject(dir)
	assert.False(t, got.Detected)
	assert.Empty(t, got.Framework)
	assert.Empty(t, got.Type)
}

// ── DetectProject – SPA ───────────────────────────────────────────────────────

// auth0 qs setup --app --type spa --framework react --build-tool vite
func TestDetectProject_React(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "vite.config.ts", "")
	writeTestFile(t, dir, "package.json", `{"name":"my-react-app","dependencies":{"react":"^18"}}`)

	got := DetectProject(dir)
	assert.True(t, got.Detected)
	assert.Equal(t, "react", got.Framework)
	assert.Equal(t, "spa", got.Type)
	assert.Equal(t, "vite", got.BuildTool)
	assert.Equal(t, 5173, got.Port)
	assert.Equal(t, "my-react-app", got.AppName)
}

// auth0 qs setup --app --type spa --framework angular
func TestDetectProject_Angular(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "angular.json", `{}`)

	got := DetectProject(dir)
	assert.True(t, got.Detected)
	assert.Equal(t, "angular", got.Framework)
	assert.Equal(t, "spa", got.Type)
	assert.Empty(t, got.BuildTool)
	assert.Equal(t, 4200, got.Port)
}

// auth0 qs setup --app --type spa --framework vue --build-tool vite
func TestDetectProject_Vue(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "vite.config.js", "")
	writeTestFile(t, dir, "package.json", `{"dependencies":{"vue":"^3"}}`)

	got := DetectProject(dir)
	assert.True(t, got.Detected)
	assert.Equal(t, "vue", got.Framework)
	assert.Equal(t, "spa", got.Type)
	assert.Equal(t, "vite", got.BuildTool)
	assert.Equal(t, 5173, got.Port)
}

// auth0 qs setup --app --type spa --framework svelte --build-tool vite
func TestDetectProject_Svelte(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "vite.config.ts", "")
	writeTestFile(t, dir, "package.json", `{"dependencies":{"svelte":"^4"}}`)

	got := DetectProject(dir)
	assert.True(t, got.Detected)
	assert.Equal(t, "svelte", got.Framework)
	assert.Equal(t, "spa", got.Type)
	assert.Equal(t, "vite", got.BuildTool)
}

// auth0 qs setup --app --type spa --framework vanilla-javascript --build-tool vite
func TestDetectProject_VanillaJavaScript(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "vite.config.ts", "")
	writeTestFile(t, dir, "package.json", `{"dependencies":{"some-utility":"^1"}}`)

	got := DetectProject(dir)
	assert.True(t, got.Detected)
	assert.Equal(t, "vanilla-javascript", got.Framework)
	assert.Equal(t, "spa", got.Type)
	assert.Equal(t, "vite", got.BuildTool)
}

func TestDetectProject_VanillaJavaScript_NoPackageJSON(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "vite.config.js", "")
	// no package.json -> deps are empty -> falls through to vanilla-javascript

	got := DetectProject(dir)
	assert.True(t, got.Detected)
	assert.Equal(t, "vanilla-javascript", got.Framework)
	assert.Equal(t, "spa", got.Type)
}

// auth0 qs setup --app --type spa --framework flutter-web
func TestDetectProject_FlutterWeb(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "pubspec.yaml", "name: my_flutter_web\nflutter:\n  sdk: flutter\n")
	mkTestDir(t, dir, "web")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "web", "index.html"), []byte("<html></html>"), 0600))

	got := DetectProject(dir)
	assert.True(t, got.Detected)
	assert.Equal(t, "flutter-web", got.Framework)
	assert.Equal(t, "spa", got.Type)
}

// pubspec.yaml without web/ dir -> native flutter
func TestDetectProject_Flutter_WithoutWeb(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "pubspec.yaml", "name: my_flutter_app\nflutter:\n  sdk: flutter\n")
	// no web/index.html

	got := DetectProject(dir)
	assert.True(t, got.Detected)
	assert.Equal(t, "flutter", got.Framework)
	assert.Equal(t, "native", got.Type)
}

// pubspec.yaml without sdk: flutter is not detected
func TestDetectProject_PubspecWithoutFlutter(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "pubspec.yaml", "name: dart_only\nversion: 1.0.0\n")

	got := DetectProject(dir)
	assert.False(t, got.Detected)
}

// ── DetectProject – Regular Web Apps ─────────────────────────────────────────

// auth0 qs setup --app --type regular --framework nextjs
func TestDetectProject_NextJS_ConfigJS(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "next.config.js", "")

	got := DetectProject(dir)
	assert.True(t, got.Detected)
	assert.Equal(t, "nextjs", got.Framework)
	assert.Equal(t, "regular", got.Type)
	assert.Equal(t, 3000, got.Port)
}

func TestDetectProject_NextJS_ConfigTS(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "next.config.ts", "")

	got := DetectProject(dir)
	assert.Equal(t, "nextjs", got.Framework)
	assert.Equal(t, "regular", got.Type)
}

func TestDetectProject_NextJS_ConfigMJS(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "next.config.mjs", "")

	got := DetectProject(dir)
	assert.Equal(t, "nextjs", got.Framework)
}

// auth0 qs setup --app --type regular --framework nuxt
func TestDetectProject_Nuxt_ConfigTS(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "nuxt.config.ts", "")

	got := DetectProject(dir)
	assert.True(t, got.Detected)
	assert.Equal(t, "nuxt", got.Framework)
	assert.Equal(t, "regular", got.Type)
	assert.Equal(t, 3000, got.Port)
}

func TestDetectProject_Nuxt_ConfigJS(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "nuxt.config.js", "")

	got := DetectProject(dir)
	assert.Equal(t, "nuxt", got.Framework)
}

// auth0 qs setup --app --type regular --framework sveltekit
func TestDetectProject_SvelteKit_ConfigJS(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "svelte.config.js", "")

	got := DetectProject(dir)
	assert.True(t, got.Detected)
	assert.Equal(t, "sveltekit", got.Framework)
	assert.Equal(t, "regular", got.Type)
}

func TestDetectProject_SvelteKit_ConfigTS(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "svelte.config.ts", "")

	got := DetectProject(dir)
	assert.Equal(t, "sveltekit", got.Framework)
	assert.Equal(t, "regular", got.Type)
}

// auth0 qs setup --app --type regular --framework fastify
func TestDetectProject_Fastify(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "package.json", `{"dependencies":{"fastify":"^4"}}`)

	got := DetectProject(dir)
	assert.True(t, got.Detected)
	assert.Equal(t, "fastify", got.Framework)
	assert.Equal(t, "regular", got.Type)
	assert.Equal(t, 3000, got.Port)
}

// auth0 qs setup --name express-app --api ... --app --type regular --framework express
func TestDetectProject_Express(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "package.json", `{"dependencies":{"express":"^4"}}`)

	got := DetectProject(dir)
	assert.True(t, got.Detected)
	assert.Equal(t, "express", got.Framework)
	assert.Equal(t, "regular", got.Type)
	assert.Equal(t, 3000, got.Port)
}

// auth0 qs setup --app --type regular --framework hono
func TestDetectProject_Hono(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "package.json", `{"dependencies":{"hono":"^3"}}`)

	got := DetectProject(dir)
	assert.True(t, got.Detected)
	assert.Equal(t, "hono", got.Framework)
	assert.Equal(t, "regular", got.Type)
	assert.Equal(t, 3000, got.Port)
}

// auth0 qs setup --app --type regular --framework vanilla-python
func TestDetectProject_VanillaPython_RequirementsTxt(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "requirements.txt", "flask==2.0\nwerkzeug\n")

	got := DetectProject(dir)
	assert.True(t, got.Detected)
	assert.Equal(t, "vanilla-python", got.Framework)
	assert.Equal(t, "regular", got.Type)
	assert.Equal(t, 5000, got.Port)
}

func TestDetectProject_VanillaPython_Pyproject(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "pyproject.toml", "[project]\nname = \"myapp\"\ndependencies = [\"Flask>=2.0\"]\n")

	got := DetectProject(dir)
	assert.True(t, got.Detected)
	assert.Equal(t, "vanilla-python", got.Framework)
	assert.Equal(t, "regular", got.Type)
}

func TestDetectProject_VanillaPython_CaseInsensitive(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "requirements.txt", "Flask==2.0\n")

	got := DetectProject(dir)
	assert.Equal(t, "vanilla-python", got.Framework)
}

// auth0 qs setup --app --type regular --framework vanilla-go
func TestDetectProject_VanillaGo(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "go.mod", "module github.com/my-org/my-service\n\ngo 1.21\n")

	got := DetectProject(dir)
	assert.True(t, got.Detected)
	assert.Equal(t, "vanilla-go", got.Framework)
	assert.Equal(t, "regular", got.Type)
}

// auth0 qs setup --app --type regular --framework rails
func TestDetectProject_Rails(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "Gemfile", "source 'https://rubygems.org'\ngem 'rails', '~> 7.0'\n")

	got := DetectProject(dir)
	assert.True(t, got.Detected)
	assert.Equal(t, "rails", got.Framework)
	assert.Equal(t, "regular", got.Type)
	assert.Equal(t, 3000, got.Port)
}

func TestDetectProject_GemfileWithoutRails(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "Gemfile", "source 'https://rubygems.org'\ngem 'sinatra'\n")

	got := DetectProject(dir)
	assert.False(t, got.Detected)
}

// auth0 qs setup --app --type regular --framework vanilla-java (pom.xml)
func TestDetectProject_VanillaJava_Maven(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "pom.xml", `<project><artifactId>my-app</artifactId></project>`)

	got := DetectProject(dir)
	assert.True(t, got.Detected)
	assert.Equal(t, "vanilla-java", got.Framework)
	assert.Equal(t, "regular", got.Type)
	assert.Equal(t, "maven", got.BuildTool)
}

// auth0 qs setup --app --type regular --framework java-ee
func TestDetectProject_JavaEE_JaxServlet(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "pom.xml", `<project><dependency><groupId>javax.servlet</groupId></dependency></project>`)

	got := DetectProject(dir)
	assert.True(t, got.Detected)
	assert.Equal(t, "java-ee", got.Framework)
	assert.Equal(t, "regular", got.Type)
	assert.Equal(t, "maven", got.BuildTool)
}

func TestDetectProject_JavaEE_JakartaEE(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "pom.xml", `<project><dependency><groupId>jakarta.ee</groupId></dependency></project>`)

	got := DetectProject(dir)
	assert.Equal(t, "java-ee", got.Framework)
}

func TestDetectProject_JavaEE_JakartaServlet(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "pom.xml", `<project><dependency><groupId>jakarta.servlet</groupId></dependency></project>`)

	got := DetectProject(dir)
	assert.Equal(t, "java-ee", got.Framework)
}

func TestDetectProject_JavaEE_JaxEE(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "pom.xml", `<project><dependency><groupId>javax.ee</groupId></dependency></project>`)

	got := DetectProject(dir)
	assert.Equal(t, "java-ee", got.Framework)
}

// auth0 qs setup --app --type regular --framework spring-boot
func TestDetectProject_SpringBoot_Maven(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "pom.xml", `<project><parent><artifactId>spring-boot-starter-parent</artifactId></parent></project>`)

	got := DetectProject(dir)
	assert.True(t, got.Detected)
	assert.Equal(t, "spring-boot", got.Framework)
	assert.Equal(t, "regular", got.Type)
	assert.Equal(t, "maven", got.BuildTool)
	assert.Equal(t, 8080, got.Port)
}

func TestDetectProject_SpringBoot_Gradle(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "build.gradle", `dependencies { implementation 'org.springframework.boot:spring-boot-starter-web' }`)

	got := DetectProject(dir)
	assert.Equal(t, "spring-boot", got.Framework)
	assert.Equal(t, "gradle", got.BuildTool)
}

func TestDetectProject_VanillaJava_GradleKts(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "build.gradle.kts", `plugins { java }`)

	got := DetectProject(dir)
	assert.True(t, got.Detected)
	assert.Equal(t, "vanilla-java", got.Framework)
	assert.Equal(t, "gradle", got.BuildTool)
}

// auth0 qs setup --app --type regular --framework aspnet-mvc
func TestDetectProject_AspnetMVC(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "MyApp.csproj",
		`<Project Sdk="Microsoft.NET.Sdk.Web"><ItemGroup><PackageReference Include="Microsoft.AspNetCore.Mvc" /></ItemGroup></Project>`)

	got := DetectProject(dir)
	assert.True(t, got.Detected)
	assert.Equal(t, "aspnet-mvc", got.Framework)
	assert.Equal(t, "regular", got.Type)
}

// auth0 qs setup --app --type regular --framework aspnet-blazor
func TestDetectProject_AspnetBlazor(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "MyApp.csproj",
		`<Project Sdk="Microsoft.NET.Sdk.Web"><ItemGroup><PackageReference Include="Microsoft.AspNetCore.Components.WebAssembly" /></ItemGroup></Project>`)

	got := DetectProject(dir)
	assert.True(t, got.Detected)
	assert.Equal(t, "aspnet-blazor", got.Framework)
	assert.Equal(t, "regular", got.Type)
}

// auth0 qs setup --app --type regular --framework aspnet-owin
func TestDetectProject_AspnetOwin(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "MyApp.csproj",
		`<Project><ItemGroup><PackageReference Include="Microsoft.Owin.Host.SystemWeb" /></ItemGroup></Project>`)

	got := DetectProject(dir)
	assert.True(t, got.Detected)
	assert.Equal(t, "aspnet-owin", got.Framework)
	assert.Equal(t, "regular", got.Type)
}

// auth0 qs setup --app --type regular --framework vanilla-php
func TestDetectProject_VanillaPHP(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "composer.json", `{"name":"my/app","require":{"php":"^8.0"}}`)

	got := DetectProject(dir)
	assert.True(t, got.Detected)
	assert.Equal(t, "vanilla-php", got.Framework)
	assert.Equal(t, "regular", got.Type)
	assert.Equal(t, "composer", got.BuildTool)
}

// auth0 qs setup --app --type regular --framework laravel
func TestDetectProject_Laravel(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "composer.json", `{"name":"my/laravel-app","require":{"laravel/framework":"^10.0"}}`)

	got := DetectProject(dir)
	assert.True(t, got.Detected)
	assert.Equal(t, "laravel", got.Framework)
	assert.Equal(t, "regular", got.Type)
	assert.Equal(t, "composer", got.BuildTool)
	assert.Equal(t, 8000, got.Port)
}

// ── DetectProject – Native / Mobile ──────────────────────────────────────────

// auth0 qs setup --app --type native --framework flutter
func TestDetectProject_Flutter(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "pubspec.yaml", "name: my_flutter_app\nflutter:\n  sdk: flutter\n")
	// no web/index.html -> native

	got := DetectProject(dir)
	assert.True(t, got.Detected)
	assert.Equal(t, "flutter", got.Framework)
	assert.Equal(t, "native", got.Type)
}

// auth0 qs setup --app --type native --framework react-native
func TestDetectProject_ReactNative(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "package.json", `{"dependencies":{"react-native":"^0.72"}}`)

	got := DetectProject(dir)
	assert.True(t, got.Detected)
	assert.Equal(t, "react-native", got.Framework)
	assert.Equal(t, "native", got.Type)
}

// auth0 qs setup --app --type native --framework expo
func TestDetectProject_Expo(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "expo.json", `{"expo":{"name":"my-expo-app"}}`)

	got := DetectProject(dir)
	assert.True(t, got.Detected)
	assert.Equal(t, "expo", got.Framework)
	assert.Equal(t, "native", got.Type)
}

// expo.json takes priority over react-native in package.json
func TestDetectProject_ExpoBeatsReactNative(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "expo.json", `{"expo":{}}`)
	writeTestFile(t, dir, "package.json", `{"dependencies":{"react-native":"^0.72"}}`)

	got := DetectProject(dir)
	assert.Equal(t, "expo", got.Framework)
}

// auth0 qs setup --app --type native --framework ionic-angular
func TestDetectProject_IonicAngular(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "package.json", `{"dependencies":{"@ionic/angular":"^7"}}`)

	got := DetectProject(dir)
	assert.True(t, got.Detected)
	assert.Equal(t, "ionic-angular", got.Framework)
	assert.Equal(t, "native", got.Type)
	assert.Empty(t, got.BuildTool)
}

// auth0 qs setup --app --type native --framework ionic-react --build-tool vite
func TestDetectProject_IonicReact(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "package.json", `{"dependencies":{"@ionic/react":"^7"}}`)

	got := DetectProject(dir)
	assert.True(t, got.Detected)
	assert.Equal(t, "ionic-react", got.Framework)
	assert.Equal(t, "native", got.Type)
	assert.Equal(t, "vite", got.BuildTool)
}

// auth0 qs setup --app --type native --framework ionic-vue --build-tool vite
func TestDetectProject_IonicVue(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "package.json", `{"dependencies":{"@ionic/vue":"^7"}}`)

	got := DetectProject(dir)
	assert.True(t, got.Detected)
	assert.Equal(t, "ionic-vue", got.Framework)
	assert.Equal(t, "native", got.Type)
	assert.Equal(t, "vite", got.BuildTool)
}

// auth0 qs setup --app --type native --framework maui (.NET Android/iOS)
func TestDetectProject_MAUI_AndroidIOS(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "MyApp.csproj",
		`<Project Sdk="Microsoft.NET.Sdk"><PropertyGroup><TargetFrameworks>net8.0-android;net8.0-ios</TargetFrameworks></PropertyGroup></Project>`)

	got := DetectProject(dir)
	assert.True(t, got.Detected)
	assert.Equal(t, "maui", got.Framework)
	assert.Equal(t, "native", got.Type)
}

func TestDetectProject_MAUI_ExplicitSDK(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "MyApp.csproj",
		`<Project Sdk="Microsoft.NET.Sdk"><PropertyGroup><UseMaui>true</UseMaui></PropertyGroup><PackageReference Include="Microsoft.Maui.Controls" /></Project>`)

	got := DetectProject(dir)
	assert.Equal(t, "maui", got.Framework)
	assert.Equal(t, "native", got.Type)
}

// auth0 qs setup --app --type native --framework wpf-winforms
func TestDetectProject_WPFWinforms(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "MyApp.csproj",
		`<Project Sdk="Microsoft.NET.Sdk"><PropertyGroup><TargetFramework>net8.0-windows</TargetFramework></PropertyGroup></Project>`)

	got := DetectProject(dir)
	assert.True(t, got.Detected)
	assert.Equal(t, "wpf-winforms", got.Framework)
	assert.Equal(t, "native", got.Type)
}

// ── DetectProject – priority rules ───────────────────────────────────────────

// angular.json beats package.json deps (checked first)
func TestDetectProject_AngularPriorityOverPackageJSON(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "angular.json", `{}`)
	writeTestFile(t, dir, "package.json", `{"dependencies":{"react":"^18"}}`)

	got := DetectProject(dir)
	assert.Equal(t, "angular", got.Framework)
}

// vite config beats package.json dep-only scan (step 3 < step 14)
func TestDetectProject_ViteConfigBeatsPackageJSONScan(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "vite.config.ts", "")
	writeTestFile(t, dir, "package.json", `{"dependencies":{"express":"^4","react":"^18"}}`)

	// vite.config.ts found first; react dep wins over express
	got := DetectProject(dir)
	assert.Equal(t, "react", got.Framework)
	assert.Equal(t, "spa", got.Type)
}

// Ambiguous: multiple package.json web deps with no config file
func TestDetectProject_AmbiguousPackageJSON(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "package.json", `{"dependencies":{"express":"^4","hono":"^3"}}`)

	got := DetectProject(dir)
	assert.True(t, got.Detected)
	assert.Empty(t, got.Framework)
	assert.Len(t, got.AmbiguousCandidates, 2)
	assert.Contains(t, got.AmbiguousCandidates, "express")
	assert.Contains(t, got.AmbiguousCandidates, "hono")
}

// ── DetectProject – app name detection ───────────────────────────────────────

func TestDetectProject_AppNameFromPackageJSON(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "vite.config.ts", "")
	writeTestFile(t, dir, "package.json", `{"name":"my-awesome-app","dependencies":{"react":"^18"}}`)

	got := DetectProject(dir)
	assert.Equal(t, "my-awesome-app", got.AppName)
}

func TestDetectProject_AppNameFromGoMod(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "go.mod", "module github.com/org/myapp\n\ngo 1.21\n")

	got := DetectProject(dir)
	assert.Equal(t, "myapp", got.AppName)
}

func TestDetectProject_AppNameFromPubspec(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "pubspec.yaml", "name: flutter_app\nflutter:\n  sdk: flutter\n")

	got := DetectProject(dir)
	assert.Equal(t, "flutter_app", got.AppName)
}

func TestDetectProject_AppNameFromComposer(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "composer.json", `{"name":"vendor/my-php-app","require":{"php":"^8"}}`)

	got := DetectProject(dir)
	assert.Equal(t, "my-php-app", got.AppName)
}

func TestDetectProject_AppNameFromPomArtifactID(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "pom.xml", `<project><groupId>com.example</groupId><artifactId>my-java-app</artifactId></project>`)

	got := DetectProject(dir)
	assert.Equal(t, "my-java-app", got.AppName)
}

// ── detectFromCsproj ──────────────────────────────────────────────────────────

func TestDetectFromCsproj(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		wantFw    string
		wantType  string
		wantFound bool
	}{
		{
			name:      "blazor",
			content:   `<PackageReference Include="Microsoft.AspNetCore.Components.WebAssembly" />`,
			wantFw:    "aspnet-blazor",
			wantType:  "regular",
			wantFound: true,
		},
		{
			name:      "mvc",
			content:   `<PackageReference Include="Microsoft.AspNetCore.Mvc" />`,
			wantFw:    "aspnet-mvc",
			wantType:  "regular",
			wantFound: true,
		},
		{
			name:      "owin",
			content:   `<PackageReference Include="Microsoft.Owin.Host.SystemWeb" />`,
			wantFw:    "aspnet-owin",
			wantType:  "regular",
			wantFound: true,
		},
		{
			name:      "maui_sdk",
			content:   `<PackageReference Include="Microsoft.Maui.Controls" />`,
			wantFw:    "maui",
			wantType:  "native",
			wantFound: true,
		},
		{
			name:      "maui_android_target",
			content:   `<TargetFrameworks>net8.0-android</TargetFrameworks>`,
			wantFw:    "maui",
			wantType:  "native",
			wantFound: true,
		},
		{
			name:      "maui_ios_target",
			content:   `<TargetFrameworks>net8.0-ios</TargetFrameworks>`,
			wantFw:    "maui",
			wantType:  "native",
			wantFound: true,
		},
		{
			name:      "wpf_winforms_windows_target",
			content:   `<TargetFramework>net8.0-windows</TargetFramework>`,
			wantFw:    "wpf-winforms",
			wantType:  "native",
			wantFound: true,
		},
		{
			name:      "blazor_takes_priority_over_mvc",
			content:   `<PackageReference Include="Microsoft.AspNetCore.Components" /><PackageReference Include="Microsoft.AspNetCore.Mvc" />`,
			wantFw:    "aspnet-blazor",
			wantType:  "regular",
			wantFound: true,
		},
		{
			name:      "unknown_csproj",
			content:   `<Project Sdk="Microsoft.NET.Sdk"><PropertyGroup><OutputType>Exe</OutputType></PropertyGroup></Project>`,
			wantFw:    "",
			wantType:  "",
			wantFound: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fw, qsType, found := detectFromCsproj(tc.content)
			assert.Equal(t, tc.wantFw, fw)
			assert.Equal(t, tc.wantType, qsType)
			assert.Equal(t, tc.wantFound, found)
		})
	}
}

// ── detectJavaFramework ───────────────────────────────────────────────────────

func TestDetectJavaFramework(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		wantFw   string
		wantPort int
	}{
		{
			name:     "spring_boot",
			content:  `<parent><artifactId>spring-boot-starter-parent</artifactId></parent>`,
			wantFw:   "spring-boot",
			wantPort: 8080,
		},
		{
			name:     "javax_ee",
			content:  `<dependency><groupId>javax.ee</groupId></dependency>`,
			wantFw:   "java-ee",
			wantPort: 0,
		},
		{
			name:     "jakarta_ee",
			content:  `<dependency><groupId>jakarta.ee</groupId></dependency>`,
			wantFw:   "java-ee",
			wantPort: 0,
		},
		{
			name:     "javax_servlet",
			content:  `<dependency><groupId>javax.servlet</groupId></dependency>`,
			wantFw:   "java-ee",
			wantPort: 0,
		},
		{
			name:     "jakarta_servlet",
			content:  `<dependency><groupId>jakarta.servlet</groupId></dependency>`,
			wantFw:   "java-ee",
			wantPort: 0,
		},
		{
			name:     "vanilla_java_plain_pom",
			content:  `<project><artifactId>plain-java</artifactId></project>`,
			wantFw:   "vanilla-java",
			wantPort: 0,
		},
		{
			name:     "spring_boot_gradle_dependency",
			content:  `implementation("org.springframework.boot:spring-boot-starter-web")`,
			wantFw:   "spring-boot",
			wantPort: 8080,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fw, port := detectJavaFramework(tc.content)
			assert.Equal(t, tc.wantFw, fw)
			assert.Equal(t, tc.wantPort, port)
		})
	}
}

// ── collectPackageJSONCandidates ──────────────────────────────────────────────

func TestCollectPackageJSONCandidates(t *testing.T) {
	t.Run("ionic_angular", func(t *testing.T) {
		got := collectPackageJSONCandidates(map[string]bool{"@ionic/angular": true})
		require.Len(t, got, 1)
		assert.Equal(t, "ionic-angular", got[0].framework)
		assert.Equal(t, "native", got[0].qsType)
		assert.Empty(t, got[0].buildTool)
	})

	t.Run("ionic_react_has_vite_build_tool", func(t *testing.T) {
		got := collectPackageJSONCandidates(map[string]bool{"@ionic/react": true})
		require.Len(t, got, 1)
		assert.Equal(t, "ionic-react", got[0].framework)
		assert.Equal(t, "native", got[0].qsType)
		assert.Equal(t, "vite", got[0].buildTool)
	})

	t.Run("ionic_vue_has_vite_build_tool", func(t *testing.T) {
		got := collectPackageJSONCandidates(map[string]bool{"@ionic/vue": true})
		require.Len(t, got, 1)
		assert.Equal(t, "ionic-vue", got[0].framework)
		assert.Equal(t, "vite", got[0].buildTool)
	})

	t.Run("react_native", func(t *testing.T) {
		got := collectPackageJSONCandidates(map[string]bool{"react-native": true})
		require.Len(t, got, 1)
		assert.Equal(t, "react-native", got[0].framework)
		assert.Equal(t, "native", got[0].qsType)
	})

	t.Run("express", func(t *testing.T) {
		got := collectPackageJSONCandidates(map[string]bool{"express": true})
		require.Len(t, got, 1)
		assert.Equal(t, "express", got[0].framework)
		assert.Equal(t, "regular", got[0].qsType)
		assert.Equal(t, 3000, got[0].port)
	})

	t.Run("hono", func(t *testing.T) {
		got := collectPackageJSONCandidates(map[string]bool{"hono": true})
		require.Len(t, got, 1)
		assert.Equal(t, "hono", got[0].framework)
		assert.Equal(t, "regular", got[0].qsType)
		assert.Equal(t, 3000, got[0].port)
	})

	t.Run("fastify", func(t *testing.T) {
		got := collectPackageJSONCandidates(map[string]bool{"fastify": true})
		require.Len(t, got, 1)
		assert.Equal(t, "fastify", got[0].framework)
		assert.Equal(t, "regular", got[0].qsType)
		assert.Equal(t, 3000, got[0].port)
	})

	t.Run("empty_deps_returns_no_candidates", func(t *testing.T) {
		got := collectPackageJSONCandidates(map[string]bool{})
		assert.Empty(t, got)
	})

	t.Run("multiple_deps_returns_multiple_candidates", func(t *testing.T) {
		deps := map[string]bool{"express": true, "hono": true, "fastify": true}
		got := collectPackageJSONCandidates(deps)
		assert.Len(t, got, 3)
	})

	t.Run("unrecognised_dep_returns_no_candidates", func(t *testing.T) {
		got := collectPackageJSONCandidates(map[string]bool{"some-random-lib": true})
		assert.Empty(t, got)
	})
}

// ── detectionFriendlyAppType ──────────────────────────────────────────────────

func TestDetectionFriendlyAppType(t *testing.T) {
	assert.Equal(t, "Single Page App", detectionFriendlyAppType("spa"))
	assert.Equal(t, "Regular Web App", detectionFriendlyAppType("regular"))
	assert.Equal(t, "Native / Mobile", detectionFriendlyAppType("native"))
	assert.Equal(t, "Machine to Machine", detectionFriendlyAppType("m2m"))
	assert.Equal(t, "unknown-type", detectionFriendlyAppType("unknown-type"))
	assert.Equal(t, "", detectionFriendlyAppType(""))
}

// ── readGoModuleName ──────────────────────────────────────────────────────────

func TestReadGoModuleName(t *testing.T) {
	t.Run("returns last path segment", func(t *testing.T) {
		dir := t.TempDir()
		writeTestFile(t, dir, "go.mod", "module github.com/org/my-service\n\ngo 1.21\n")
		assert.Equal(t, "my-service", readGoModuleName(dir))
	})

	t.Run("bare module name", func(t *testing.T) {
		dir := t.TempDir()
		writeTestFile(t, dir, "go.mod", "module myapp\n\ngo 1.21\n")
		assert.Equal(t, "myapp", readGoModuleName(dir))
	})

	t.Run("no go.mod returns empty", func(t *testing.T) {
		assert.Empty(t, readGoModuleName(t.TempDir()))
	})
}

// ── readPyprojectName ─────────────────────────────────────────────────────────

func TestReadPyprojectName(t *testing.T) {
	t.Run("reads project name", func(t *testing.T) {
		dir := t.TempDir()
		writeTestFile(t, dir, "pyproject.toml", "[project]\nname = \"my-python-app\"\nversion = \"0.1\"\n")
		assert.Equal(t, "my-python-app", readPyprojectName(dir))
	})

	t.Run("no pyproject.toml returns empty", func(t *testing.T) {
		assert.Empty(t, readPyprojectName(t.TempDir()))
	})
}

// ── readPubspecName ───────────────────────────────────────────────────────────

func TestReadPubspecName(t *testing.T) {
	t.Run("reads name field", func(t *testing.T) {
		dir := t.TempDir()
		writeTestFile(t, dir, "pubspec.yaml", "name: flutter_app\nversion: 1.0.0\n")
		assert.Equal(t, "flutter_app", readPubspecName(dir))
	})

	t.Run("no pubspec.yaml returns empty", func(t *testing.T) {
		assert.Empty(t, readPubspecName(t.TempDir()))
	})
}

// ── readComposerName ──────────────────────────────────────────────────────────

func TestReadComposerName(t *testing.T) {
	t.Run("returns part after slash", func(t *testing.T) {
		dir := t.TempDir()
		writeTestFile(t, dir, "composer.json", `{"name":"vendor/my-php-app"}`)
		assert.Equal(t, "my-php-app", readComposerName(dir))
	})

	t.Run("name without slash", func(t *testing.T) {
		dir := t.TempDir()
		writeTestFile(t, dir, "composer.json", `{"name":"myapp"}`)
		assert.Equal(t, "myapp", readComposerName(dir))
	})

	t.Run("no composer.json returns empty", func(t *testing.T) {
		assert.Empty(t, readComposerName(t.TempDir()))
	})
}

// ── readPomArtifactID ─────────────────────────────────────────────────────────

func TestReadPomArtifactID(t *testing.T) {
	t.Run("reads first artifactId", func(t *testing.T) {
		dir := t.TempDir()
		writeTestFile(t, dir, "pom.xml",
			`<project><groupId>com.example</groupId><artifactId>my-java-app</artifactId></project>`)
		assert.Equal(t, "my-java-app", readPomArtifactID(dir))
	})

	t.Run("no pom.xml returns empty", func(t *testing.T) {
		assert.Empty(t, readPomArtifactID(t.TempDir()))
	})

	t.Run("pom without artifactId returns empty", func(t *testing.T) {
		dir := t.TempDir()
		writeTestFile(t, dir, "pom.xml", `<project><groupId>com.example</groupId></project>`)
		assert.Empty(t, readPomArtifactID(dir))
	})
}

// ── readPackageJSONName ───────────────────────────────────────────────────────

func TestReadPackageJSONName(t *testing.T) {
	t.Run("reads name field", func(t *testing.T) {
		dir := t.TempDir()
		writeTestFile(t, dir, "package.json", `{"name":"my-js-app","version":"1.0.0"}`)
		assert.Equal(t, "my-js-app", readPackageJSONName(dir))
	})

	t.Run("no package.json returns empty", func(t *testing.T) {
		assert.Empty(t, readPackageJSONName(t.TempDir()))
	})

	t.Run("invalid json returns empty", func(t *testing.T) {
		dir := t.TempDir()
		writeTestFile(t, dir, "package.json", `not valid json`)
		assert.Empty(t, readPackageJSONName(dir))
	})
}

// ── defaultPortForFramework ───────────────────────────────────────────────────

func TestDefaultPortForFramework(t *testing.T) {
	tests := []struct {
		framework string
		wantPort  int
	}{
		// SPA vite frameworks
		{"react", 5173},
		{"vue", 5173},
		{"svelte", 5173},
		{"vanilla-javascript", 5173},
		// SPA non-vite
		{"angular", 4200},
		// Regular – Python
		{"vanilla-python", 5000},
		{"flask", 5000},
		// Regular – PHP
		{"laravel", 8000},
		// Regular – Java
		{"spring-boot", 8080},
		{"java-ee", 8080},
		{"vanilla-java", 8080},
		// Regular – default 3000
		{"nextjs", 3000},
		{"nuxt", 3000},
		{"express", 3000},
		{"fastify", 3000},
		{"hono", 3000},
		{"sveltekit", 3000},
		{"rails", 3000},
		{"vanilla-go", 3000},
		{"django", 3000},
		// Native – default 3000
		{"flutter", 3000},
		{"react-native", 3000},
		{"expo", 3000},
		// Catch-all
		{"unknown-framework", 3000},
	}

	for _, tc := range tests {
		t.Run(tc.framework, func(t *testing.T) {
			assert.Equal(t, tc.wantPort, defaultPortForFramework(tc.framework))
		})
	}
}

// ── frameworksForType ─────────────────────────────────────────────────────────

func TestFrameworksForType(t *testing.T) {
	t.Run("spa", func(t *testing.T) {
		fws := frameworksForType("spa")
		assert.Contains(t, fws, "react")
		assert.Contains(t, fws, "angular")
		assert.Contains(t, fws, "vue")
		assert.Contains(t, fws, "svelte")
		assert.Contains(t, fws, "vanilla-javascript")
		assert.Contains(t, fws, "flutter-web")
		// SPA frameworks must be sorted
		assert.Equal(t, sort.StringsAreSorted(fws), true)
	})

	t.Run("regular", func(t *testing.T) {
		fws := frameworksForType("regular")
		assert.Contains(t, fws, "nextjs")
		assert.Contains(t, fws, "nuxt")
		assert.Contains(t, fws, "fastify")
		assert.Contains(t, fws, "sveltekit")
		assert.Contains(t, fws, "express")
		assert.Contains(t, fws, "hono")
		assert.Contains(t, fws, "vanilla-python")
		assert.Contains(t, fws, "django")
		assert.Contains(t, fws, "vanilla-go")
		assert.Contains(t, fws, "vanilla-java")
		assert.Contains(t, fws, "java-ee")
		assert.Contains(t, fws, "spring-boot")
		assert.Contains(t, fws, "aspnet-mvc")
		assert.Contains(t, fws, "aspnet-blazor")
		assert.Contains(t, fws, "aspnet-owin")
		assert.Contains(t, fws, "vanilla-php")
		assert.Contains(t, fws, "laravel")
		assert.Contains(t, fws, "rails")
	})

	t.Run("native", func(t *testing.T) {
		fws := frameworksForType("native")
		assert.Contains(t, fws, "flutter")
		assert.Contains(t, fws, "react-native")
		assert.Contains(t, fws, "expo")
		assert.Contains(t, fws, "ionic-angular")
		assert.Contains(t, fws, "ionic-react")
		assert.Contains(t, fws, "ionic-vue")
		assert.Contains(t, fws, "dotnet-mobile")
		assert.Contains(t, fws, "maui")
		assert.Contains(t, fws, "wpf-winforms")
	})

	t.Run("unknown type returns empty", func(t *testing.T) {
		assert.Empty(t, frameworksForType("nonexistent"))
	})
}

// ── getQuickstartConfigKey ────────────────────────────────────────────────────
//
// Tests cover all framework/type/buildTool combinations from the requirements
// table. All inputs are fully populated to avoid interactive prompts.

func TestGetQuickstartConfigKey(t *testing.T) {
	tests := []struct {
		name           string
		inputs         SetupInputs
		wantKey        string
		wantBuildTool  string
		wantAutoSelect bool
	}{
		// ── SPA ──────────────────────────────────────────────────────────────
		// auth0 qs setup --app --type spa --framework react --build-tool vite
		{
			name:          "spa react vite",
			inputs:        SetupInputs{App: true, Type: "spa", Framework: "react", BuildTool: "vite", Port: 5173},
			wantKey:       "spa:react:vite",
			wantBuildTool: "vite",
		},
		{
			name:           "spa react build-tool none auto-selects vite",
			inputs:         SetupInputs{App: true, Type: "spa", Framework: "react", BuildTool: "none", Port: 5173},
			wantKey:        "spa:react:vite",
			wantBuildTool:  "vite",
			wantAutoSelect: true,
		},
		// auth0 qs setup --app --type spa --framework angular
		{
			name:          "spa angular none",
			inputs:        SetupInputs{App: true, Type: "spa", Framework: "angular", BuildTool: "none", Port: 4200},
			wantKey:       "spa:angular:none",
			wantBuildTool: "none",
		},
		// auth0 qs setup --app --type spa --framework vue --build-tool vite
		{
			name:          "spa vue vite",
			inputs:        SetupInputs{App: true, Type: "spa", Framework: "vue", BuildTool: "vite", Port: 5173},
			wantKey:       "spa:vue:vite",
			wantBuildTool: "vite",
		},
		// auth0 qs setup --app --type spa --framework svelte --build-tool vite
		{
			name:          "spa svelte vite",
			inputs:        SetupInputs{App: true, Type: "spa", Framework: "svelte", BuildTool: "vite", Port: 5173},
			wantKey:       "spa:svelte:vite",
			wantBuildTool: "vite",
		},
		// auth0 qs setup --app --type spa --framework vanilla-javascript --build-tool vite
		{
			name:          "spa vanilla-javascript vite",
			inputs:        SetupInputs{App: true, Type: "spa", Framework: "vanilla-javascript", BuildTool: "vite", Port: 5173},
			wantKey:       "spa:vanilla-javascript:vite",
			wantBuildTool: "vite",
		},
		// auth0 qs setup --app --type spa --framework flutter-web
		{
			name:          "spa flutter-web none",
			inputs:        SetupInputs{App: true, Type: "spa", Framework: "flutter-web", BuildTool: "none", Port: 3000},
			wantKey:       "spa:flutter-web:none",
			wantBuildTool: "none",
		},

		// ── Regular ──────────────────────────────────────────────────────────
		// auth0 qs setup --app --type regular --framework nextjs
		{
			name:          "regular nextjs none",
			inputs:        SetupInputs{App: true, Type: "regular", Framework: "nextjs", BuildTool: "none", Port: 3000},
			wantKey:       "regular:nextjs:none",
			wantBuildTool: "none",
		},
		// auth0 qs setup --app --type regular --framework nuxt
		{
			name:          "regular nuxt none",
			inputs:        SetupInputs{App: true, Type: "regular", Framework: "nuxt", BuildTool: "none", Port: 3000},
			wantKey:       "regular:nuxt:none",
			wantBuildTool: "none",
		},
		// auth0 qs setup --app --type regular --framework fastify
		{
			name:          "regular fastify none",
			inputs:        SetupInputs{App: true, Type: "regular", Framework: "fastify", BuildTool: "none", Port: 3000},
			wantKey:       "regular:fastify:none",
			wantBuildTool: "none",
		},
		// auth0 qs setup --app --type regular --framework sveltekit
		{
			name:          "regular sveltekit none",
			inputs:        SetupInputs{App: true, Type: "regular", Framework: "sveltekit", BuildTool: "none", Port: 3000},
			wantKey:       "regular:sveltekit:none",
			wantBuildTool: "none",
		},
		// auth0 qs setup --name express-app --api ... --app --type regular --framework express
		{
			name:          "regular express none",
			inputs:        SetupInputs{App: true, Type: "regular", Framework: "express", BuildTool: "none", Port: 3000},
			wantKey:       "regular:express:none",
			wantBuildTool: "none",
		},
		// auth0 qs setup --app --type regular --framework hono
		{
			name:          "regular hono none",
			inputs:        SetupInputs{App: true, Type: "regular", Framework: "hono", BuildTool: "none", Port: 3000},
			wantKey:       "regular:hono:none",
			wantBuildTool: "none",
		},
		// auth0 qs setup --app --type regular --framework vanilla-python
		{
			name:          "regular vanilla-python none",
			inputs:        SetupInputs{App: true, Type: "regular", Framework: "vanilla-python", BuildTool: "none", Port: 5000},
			wantKey:       "regular:vanilla-python:none",
			wantBuildTool: "none",
		},
		// auth0 qs setup --app --type regular --framework django
		{
			name:          "regular django none",
			inputs:        SetupInputs{App: true, Type: "regular", Framework: "django", BuildTool: "none", Port: 3000},
			wantKey:       "regular:django:none",
			wantBuildTool: "none",
		},
		// auth0 qs setup --app --type regular --framework vanilla-go
		{
			name:          "regular vanilla-go none",
			inputs:        SetupInputs{App: true, Type: "regular", Framework: "vanilla-go", BuildTool: "none", Port: 3000},
			wantKey:       "regular:vanilla-go:none",
			wantBuildTool: "none",
		},
		// auth0 qs setup --app --type regular --framework vanilla-java
		{
			name:          "regular vanilla-java maven",
			inputs:        SetupInputs{App: true, Type: "regular", Framework: "vanilla-java", BuildTool: "maven", Port: 8080},
			wantKey:       "regular:vanilla-java:maven",
			wantBuildTool: "maven",
		},
		// auth0 qs setup --app --type regular --framework java-ee
		{
			name:          "regular java-ee maven",
			inputs:        SetupInputs{App: true, Type: "regular", Framework: "java-ee", BuildTool: "maven", Port: 8080},
			wantKey:       "regular:java-ee:maven",
			wantBuildTool: "maven",
		},
		// auth0 qs setup --app --type regular --framework spring-boot
		{
			name:          "regular spring-boot maven",
			inputs:        SetupInputs{App: true, Type: "regular", Framework: "spring-boot", BuildTool: "maven", Port: 8080},
			wantKey:       "regular:spring-boot:maven",
			wantBuildTool: "maven",
		},
		// auth0 qs setup --app --type regular --framework aspnet-mvc
		{
			name:          "regular aspnet-mvc none",
			inputs:        SetupInputs{App: true, Type: "regular", Framework: "aspnet-mvc", BuildTool: "none", Port: 3000},
			wantKey:       "regular:aspnet-mvc:none",
			wantBuildTool: "none",
		},
		// auth0 qs setup --app --type regular --framework aspnet-blazor
		{
			name:          "regular aspnet-blazor none",
			inputs:        SetupInputs{App: true, Type: "regular", Framework: "aspnet-blazor", BuildTool: "none", Port: 3000},
			wantKey:       "regular:aspnet-blazor:none",
			wantBuildTool: "none",
		},
		// auth0 qs setup --app --type regular --framework aspnet-owin
		{
			name:          "regular aspnet-owin none",
			inputs:        SetupInputs{App: true, Type: "regular", Framework: "aspnet-owin", BuildTool: "none", Port: 3000},
			wantKey:       "regular:aspnet-owin:none",
			wantBuildTool: "none",
		},
		// auth0 qs setup --app --type regular --framework vanilla-php
		{
			name:          "regular vanilla-php composer",
			inputs:        SetupInputs{App: true, Type: "regular", Framework: "vanilla-php", BuildTool: "composer", Port: 3000},
			wantKey:       "regular:vanilla-php:composer",
			wantBuildTool: "composer",
		},
		// auth0 qs setup --app --type regular --framework laravel
		{
			name:          "regular laravel composer",
			inputs:        SetupInputs{App: true, Type: "regular", Framework: "laravel", BuildTool: "composer", Port: 8000},
			wantKey:       "regular:laravel:composer",
			wantBuildTool: "composer",
		},
		// auth0 qs setup --app --type regular --framework rails
		{
			name:          "regular rails none",
			inputs:        SetupInputs{App: true, Type: "regular", Framework: "rails", BuildTool: "none", Port: 3000},
			wantKey:       "regular:rails:none",
			wantBuildTool: "none",
		},

		// ── Native ───────────────────────────────────────────────────────────
		// auth0 qs setup --app --type native --framework flutter
		{
			name:          "native flutter none",
			inputs:        SetupInputs{App: true, Type: "native", Framework: "flutter", BuildTool: "none", Port: 3000},
			wantKey:       "native:flutter:none",
			wantBuildTool: "none",
		},
		// auth0 qs setup --app --type native --framework react-native
		{
			name:          "native react-native none",
			inputs:        SetupInputs{App: true, Type: "native", Framework: "react-native", BuildTool: "none", Port: 3000},
			wantKey:       "native:react-native:none",
			wantBuildTool: "none",
		},
		// auth0 qs setup --app --type native --framework expo
		{
			name:          "native expo none",
			inputs:        SetupInputs{App: true, Type: "native", Framework: "expo", BuildTool: "none", Port: 3000},
			wantKey:       "native:expo:none",
			wantBuildTool: "none",
		},
		// auth0 qs setup --app --type native --framework ionic-angular
		{
			name:          "native ionic-angular none",
			inputs:        SetupInputs{App: true, Type: "native", Framework: "ionic-angular", BuildTool: "none", Port: 3000},
			wantKey:       "native:ionic-angular:none",
			wantBuildTool: "none",
		},
		// auth0 qs setup --app --type native --framework ionic-react --build-tool vite
		{
			name:          "native ionic-react vite",
			inputs:        SetupInputs{App: true, Type: "native", Framework: "ionic-react", BuildTool: "vite", Port: 3000},
			wantKey:       "native:ionic-react:vite",
			wantBuildTool: "vite",
		},
		// auth0 qs setup --app --type native --framework ionic-vue --build-tool vite
		{
			name:          "native ionic-vue vite",
			inputs:        SetupInputs{App: true, Type: "native", Framework: "ionic-vue", BuildTool: "vite", Port: 3000},
			wantKey:       "native:ionic-vue:vite",
			wantBuildTool: "vite",
		},
		// auth0 qs setup --app --type native --framework dotnet-mobile
		{
			name:          "native dotnet-mobile none",
			inputs:        SetupInputs{App: true, Type: "native", Framework: "dotnet-mobile", BuildTool: "none", Port: 3000},
			wantKey:       "native:dotnet-mobile:none",
			wantBuildTool: "none",
		},
		// auth0 qs setup --app --type native --framework maui
		{
			name:          "native maui none",
			inputs:        SetupInputs{App: true, Type: "native", Framework: "maui", BuildTool: "none", Port: 3000},
			wantKey:       "native:maui:none",
			wantBuildTool: "none",
		},
		// auth0 qs setup --app --type native --framework wpf-winforms
		{
			name:          "native wpf-winforms none",
			inputs:        SetupInputs{App: true, Type: "native", Framework: "wpf-winforms", BuildTool: "none", Port: 3000},
			wantKey:       "native:wpf-winforms:none",
			wantBuildTool: "none",
		},

		// ── API-only: no app ─────────────────────────────────────────────────
		{
			name:    "api-only returns empty key",
			inputs:  SetupInputs{App: false, API: true},
			wantKey: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			key, updated, wasAuto, err := getQuickstartConfigKey(tc.inputs)
			require.NoError(t, err)
			assert.Equal(t, tc.wantKey, key)
			assert.Equal(t, tc.wantAutoSelect, wasAuto)
			if tc.inputs.App {
				assert.Equal(t, tc.wantBuildTool, updated.BuildTool)
			}
		})
	}
}

func TestGetQuickstartConfigKey_EmptyBuildToolTreatedAsNone(t *testing.T) {
	// BuildTool == "" should be normalised to "none" internally
	inputs := SetupInputs{App: true, Type: "regular", Framework: "nextjs", BuildTool: "", Port: 3000}
	key, _, _, err := getQuickstartConfigKey(inputs)
	require.NoError(t, err)
	assert.Equal(t, "regular:nextjs:none", key)
}

// ── resolveRequestParams ──────────────────────────────────────────────────────

func TestResolveRequestParams(t *testing.T) {
	const sub = auth0.DetectionSub

	t.Run("DetectionSub replaced in callbacks", func(t *testing.T) {
		req := auth0.RequestParams{
			AppType:           "spa",
			Callbacks:         []string{sub},
			AllowedLogoutURLs: []string{sub},
			WebOrigins:        []string{sub},
			Name:              sub,
		}
		got := resolveRequestParams(req, "MyApp", 3000)
		assert.Equal(t, []string{"http://localhost:3000/callback"}, got.Callbacks)
		assert.Equal(t, []string{"http://localhost:3000"}, got.AllowedLogoutURLs)
		assert.Equal(t, []string{"http://localhost:3000"}, got.WebOrigins)
		assert.Equal(t, "MyApp", got.Name)
		assert.Equal(t, "spa", got.AppType)
	})

	t.Run("port 0 defaults to 3000", func(t *testing.T) {
		req := auth0.RequestParams{Callbacks: []string{sub}}
		got := resolveRequestParams(req, "App", 0)
		assert.Equal(t, []string{"http://localhost:3000/callback"}, got.Callbacks)
	})

	t.Run("custom port is used", func(t *testing.T) {
		req := auth0.RequestParams{Callbacks: []string{sub}, AllowedLogoutURLs: []string{sub}}
		got := resolveRequestParams(req, "App", 5173)
		assert.Equal(t, []string{"http://localhost:5173/callback"}, got.Callbacks)
		assert.Equal(t, []string{"http://localhost:5173"}, got.AllowedLogoutURLs)
	})

	t.Run("literal URLs are not replaced", func(t *testing.T) {
		req := auth0.RequestParams{
			Callbacks:         []string{"http://localhost:5173/callback"},
			AllowedLogoutURLs: []string{"http://localhost:5173"},
		}
		got := resolveRequestParams(req, "App", 5173)
		assert.Equal(t, []string{"http://localhost:5173/callback"}, got.Callbacks)
		assert.Equal(t, []string{"http://localhost:5173"}, got.AllowedLogoutURLs)
	})

	t.Run("non-DetectionSub name is preserved", func(t *testing.T) {
		req := auth0.RequestParams{Name: "literal-name"}
		got := resolveRequestParams(req, "OtherName", 3000)
		assert.Equal(t, "literal-name", got.Name)
	})
}

// ── replaceDetectionSub ───────────────────────────────────────────────────────

func TestReplaceDetectionSub(t *testing.T) {
	const sub = auth0.DetectionSub
	const domain = "tenant.auth0.com"

	clientID := "test-client-id"
	clientSecret := "test-client-secret"
	client := &management.Client{
		ClientID:     &clientID,
		ClientSecret: &clientSecret,
	}

	t.Run("domain keys", func(t *testing.T) {
		domainKeys := []string{
			"VITE_AUTH0_DOMAIN",
			"AUTH0_DOMAIN",
			"NUXT_AUTH0_DOMAIN",
			"EXPO_PUBLIC_AUTH0_DOMAIN",
			"domain",
			"auth0.domain",
			"Auth0:Domain",
			"auth0:Domain",
			"auth0_domain",
		}
		for _, key := range domainKeys {
			t.Run(key, func(t *testing.T) {
				got, err := replaceDetectionSub(map[string]string{key: sub}, domain, client, 3000)
				require.NoError(t, err)
				assert.Equal(t, domain, got[key])
			})
		}
	})

	t.Run("ISSUER_BASE_URL gets https prefix", func(t *testing.T) {
		got, err := replaceDetectionSub(map[string]string{"ISSUER_BASE_URL": sub}, domain, client, 3000)
		require.NoError(t, err)
		assert.Equal(t, "https://"+domain, got["ISSUER_BASE_URL"])
	})

	t.Run("okta issuer gets https prefix and trailing slash", func(t *testing.T) {
		got, err := replaceDetectionSub(map[string]string{"okta.oauth2.issuer": sub}, domain, client, 3000)
		require.NoError(t, err)
		assert.Equal(t, "https://"+domain+"/", got["okta.oauth2.issuer"])
	})

	t.Run("client ID keys", func(t *testing.T) {
		clientIDKeys := []string{
			"VITE_AUTH0_CLIENT_ID",
			"AUTH0_CLIENT_ID",
			"CLIENT_ID",
			"EXPO_PUBLIC_AUTH0_CLIENT_ID",
			"NUXT_AUTH0_CLIENT_ID",
			"clientId",
			"auth0.clientId",
			"okta.oauth2.client-id",
			"Auth0:ClientId",
			"auth0:ClientId",
			"auth0_client_id",
		}
		for _, key := range clientIDKeys {
			t.Run(key, func(t *testing.T) {
				got, err := replaceDetectionSub(map[string]string{key: sub}, domain, client, 3000)
				require.NoError(t, err)
				assert.Equal(t, clientID, got[key])
			})
		}
	})

	t.Run("client secret keys", func(t *testing.T) {
		secretKeys := []string{
			"AUTH0_CLIENT_SECRET",
			"NUXT_AUTH0_CLIENT_SECRET",
			"auth0.clientSecret",
			"okta.oauth2.client-secret",
			"Auth0:ClientSecret",
			"auth0:ClientSecret",
			"auth0_client_secret",
		}
		for _, key := range secretKeys {
			t.Run(key, func(t *testing.T) {
				got, err := replaceDetectionSub(map[string]string{key: sub}, domain, client, 3000)
				require.NoError(t, err)
				assert.Equal(t, clientSecret, got[key])
			})
		}
	})

	t.Run("secret generation keys produce non-empty random value", func(t *testing.T) {
		secretGenKeys := []string{
			"AUTH0_SECRET",
			"NUXT_AUTH0_SESSION_SECRET",
			"SESSION_SECRET",
			"SECRET",
			"AUTH0_SESSION_ENCRYPTION_KEY",
			"AUTH0_COOKIE_SECRET",
		}
		for _, key := range secretGenKeys {
			t.Run(key, func(t *testing.T) {
				got, err := replaceDetectionSub(map[string]string{key: sub}, domain, client, 3000)
				require.NoError(t, err)
				assert.NotEmpty(t, got[key])
				assert.NotEqual(t, sub, got[key])
			})
		}
	})

	t.Run("base URL keys", func(t *testing.T) {
		for _, key := range []string{"APP_BASE_URL", "NUXT_AUTH0_APP_BASE_URL", "BASE_URL"} {
			t.Run(key, func(t *testing.T) {
				got, err := replaceDetectionSub(map[string]string{key: sub}, domain, client, 3000)
				require.NoError(t, err)
				assert.Equal(t, "http://localhost:3000", got[key])
			})
		}
	})

	t.Run("redirect and callback URL keys", func(t *testing.T) {
		for _, key := range []string{"AUTH0_REDIRECT_URI", "AUTH0_CALLBACK_URL"} {
			t.Run(key, func(t *testing.T) {
				got, err := replaceDetectionSub(map[string]string{key: sub}, domain, client, 5000)
				require.NoError(t, err)
				assert.Equal(t, "http://localhost:5000/callback", got[key])
			})
		}
	})

	t.Run("literal values are preserved unchanged", func(t *testing.T) {
		got, err := replaceDetectionSub(map[string]string{"SOME_KEY": "literal-value"}, domain, client, 3000)
		require.NoError(t, err)
		assert.Equal(t, "literal-value", got["SOME_KEY"])
	})

	t.Run("port 0 defaults to 3000 for URL keys", func(t *testing.T) {
		got, err := replaceDetectionSub(map[string]string{"BASE_URL": sub}, domain, client, 0)
		require.NoError(t, err)
		assert.Equal(t, "http://localhost:3000", got["BASE_URL"])
	})
}

// ── buildNestedMap ────────────────────────────────────────────────────────────

func TestBuildNestedMap(t *testing.T) {
	t.Run("dot-delimited keys produce nested structure", func(t *testing.T) {
		flat := map[string]string{
			"okta.oauth2.issuer":        "https://example.auth0.com/",
			"okta.oauth2.client-id":     "abc",
			"okta.oauth2.client-secret": "secret",
		}
		got := buildNestedMap(flat)

		okta, ok := got["okta"].(map[string]interface{})
		require.True(t, ok, "expected 'okta' to be a map")
		oauth2, ok := okta["oauth2"].(map[string]interface{})
		require.True(t, ok, "expected 'oauth2' to be a map")
		assert.Equal(t, "https://example.auth0.com/", oauth2["issuer"])
		assert.Equal(t, "abc", oauth2["client-id"])
		assert.Equal(t, "secret", oauth2["client-secret"])
	})

	t.Run("non-dot keys remain top-level", func(t *testing.T) {
		flat := map[string]string{"Domain": "example.com", "ClientId": "abc"}
		got := buildNestedMap(flat)
		assert.Equal(t, "example.com", got["Domain"])
		assert.Equal(t, "abc", got["ClientId"])
	})

	t.Run("empty map returns empty result", func(t *testing.T) {
		got := buildNestedMap(map[string]string{})
		assert.Empty(t, got)
	})
}

// ── sortedKeys ────────────────────────────────────────────────────────────────

func TestSortedKeys(t *testing.T) {
	m := map[string]string{"beta": "b", "alpha": "a", "gamma": "g", "delta": "d"}
	got := sortedKeys(m)
	assert.Equal(t, []string{"alpha", "beta", "delta", "gamma"}, got)
}

func TestSortedKeys_EmptyMap(t *testing.T) {
	assert.Empty(t, sortedKeys(map[string]string{}))
}

// ── GenerateAndWriteQuickstartConfig ──────────────────────────────────────────

func TestGenerateAndWriteQuickstartConfig(t *testing.T) {
	clientID := "cid-123"
	clientSecret := "csecret-456"
	client := &management.Client{
		ClientID:     &clientID,
		ClientSecret: &clientSecret,
	}
	const domain = "tenant.auth0.com"

	tests := []struct {
		name         string
		strategy     auth0.FileOutputStrategy
		envValues    map[string]string
		port         int
		checkContent func(t *testing.T, content string)
	}{
		// dotenv – covers React, Vue, Svelte, Vanilla JS, Next.js, Nuxt, etc.
		{
			name:     "dotenv format",
			strategy: auth0.FileOutputStrategy{Format: "dotenv"},
			envValues: map[string]string{
				"AUTH0_DOMAIN":    auth0.DetectionSub,
				"AUTH0_CLIENT_ID": auth0.DetectionSub,
			},
			port: 3000,
			checkContent: func(t *testing.T, content string) {
				assert.Contains(t, content, "AUTH0_DOMAIN=tenant.auth0.com")
				assert.Contains(t, content, "AUTH0_CLIENT_ID=cid-123")
			},
		},
		// TypeScript environment file – covers Angular, Ionic Angular
		{
			name:     "ts format",
			strategy: auth0.FileOutputStrategy{Format: "ts"},
			envValues: map[string]string{
				"domain":   auth0.DetectionSub,
				"clientId": auth0.DetectionSub,
			},
			port: 4200,
			checkContent: func(t *testing.T, content string) {
				assert.Contains(t, content, "export const environment")
				assert.Contains(t, content, "domain: 'tenant.auth0.com'")
				assert.Contains(t, content, "clientId: 'cid-123'")
			},
		},
		// Dart – covers Flutter and Flutter Web
		{
			name:     "dart format",
			strategy: auth0.FileOutputStrategy{Format: "dart"},
			envValues: map[string]string{
				"domain":   auth0.DetectionSub,
				"clientId": auth0.DetectionSub,
			},
			port: 3000,
			checkContent: func(t *testing.T, content string) {
				assert.Contains(t, content, "const Map<String, String> authConfig")
				assert.Contains(t, content, "'domain': 'tenant.auth0.com'")
				assert.Contains(t, content, "'clientId': 'cid-123'")
			},
		},
		// YAML – covers Spring Boot (application.yml)
		{
			name:     "yaml format",
			strategy: auth0.FileOutputStrategy{Format: "yaml"},
			envValues: map[string]string{
				"okta.oauth2.issuer":        auth0.DetectionSub,
				"okta.oauth2.client-id":     auth0.DetectionSub,
				"okta.oauth2.client-secret": auth0.DetectionSub,
			},
			port: 8080,
			checkContent: func(t *testing.T, content string) {
				assert.Contains(t, content, "okta:")
				assert.Contains(t, content, "oauth2:")
				assert.Contains(t, content, "https://tenant.auth0.com/")
				assert.Contains(t, content, "cid-123")
			},
		},
		// JSON – covers ASP.NET Core MVC, Blazor, dotnet-mobile, MAUI, WPF
		{
			name:     "json format",
			strategy: auth0.FileOutputStrategy{Format: "json"},
			envValues: map[string]string{
				"Auth0:Domain":   auth0.DetectionSub,
				"Auth0:ClientId": auth0.DetectionSub,
			},
			port: 3000,
			checkContent: func(t *testing.T, content string) {
				assert.Contains(t, content, `"Auth0"`)
				assert.Contains(t, content, `"Domain"`)
				assert.Contains(t, content, `"tenant.auth0.com"`)
				assert.Contains(t, content, `"ClientId"`)
				assert.Contains(t, content, `"cid-123"`)
			},
		},
		// XML – covers ASP.NET OWIN (Web.config)
		{
			name:     "xml format",
			strategy: auth0.FileOutputStrategy{Format: "xml"},
			envValues: map[string]string{
				"auth0:Domain":       auth0.DetectionSub,
				"auth0:ClientId":     auth0.DetectionSub,
				"auth0:ClientSecret": auth0.DetectionSub,
			},
			port: 3000,
			checkContent: func(t *testing.T, content string) {
				assert.Contains(t, content, `<?xml version="1.0"`)
				assert.Contains(t, content, `key="auth0:Domain"`)
				assert.Contains(t, content, `value="tenant.auth0.com"`)
				assert.Contains(t, content, `key="auth0:ClientId"`)
				assert.Contains(t, content, `value="cid-123"`)
				assert.Contains(t, content, `key="auth0:ClientSecret"`)
				assert.Contains(t, content, `value="csecret-456"`)
			},
		},
		// Properties format – covers vanilla-java, java-ee (application.properties)
		{
			name:     "properties format",
			strategy: auth0.FileOutputStrategy{Format: "properties"},
			envValues: map[string]string{
				"auth0.domain":       auth0.DetectionSub,
				"auth0.clientId":     auth0.DetectionSub,
				"auth0.clientSecret": auth0.DetectionSub,
			},
			port: 8080,
			checkContent: func(t *testing.T, content string) {
				assert.Contains(t, content, "auth0.domain=tenant.auth0.com")
				assert.Contains(t, content, "auth0.clientId=cid-123")
				assert.Contains(t, content, "auth0.clientSecret=csecret-456")
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			strategy := tc.strategy
			// Place the output file inside the temp dir so we don't pollute CWD.
			strategy.Path = filepath.Join(dir, "output_file")

			fileName, filePath, err := GenerateAndWriteQuickstartConfig(&strategy, tc.envValues, domain, client, tc.port)
			require.NoError(t, err)
			assert.NotEmpty(t, fileName)
			assert.Equal(t, strategy.Path, filePath)

			data, err := os.ReadFile(filePath)
			require.NoError(t, err)
			tc.checkContent(t, string(data))
		})
	}
}

func TestGenerateAndWriteQuickstartConfig_CreatesSubdirectory(t *testing.T) {
	dir := t.TempDir()
	clientID := "cid"
	client := &management.Client{ClientID: &clientID}

	strategy := auth0.FileOutputStrategy{
		Path:   filepath.Join(dir, "src", "environments", "environment.ts"),
		Format: "ts",
	}
	envValues := map[string]string{
		"domain":   auth0.DetectionSub,
		"clientId": auth0.DetectionSub,
	}

	_, filePath, err := GenerateAndWriteQuickstartConfig(&strategy, envValues, "tenant.auth0.com", client, 4200)
	require.NoError(t, err)

	_, statErr := os.Stat(filepath.Dir(filePath))
	assert.NoError(t, statErr, "subdirectory should have been created")
}

// ── generateClient ────────────────────────────────────────────────────────────

func TestGenerateClient(t *testing.T) {
	const sub = auth0.DetectionSub

	tests := []struct {
		name            string
		input           SetupInputs
		reqParams       auth0.RequestParams
		wantName        string
		wantAppType     string
		wantCallbacks   []string
		wantLogouts     []string
		wantWebOrigins  *[]string // nil means no WebOrigins field set
		wantOIDC        bool
		wantAlgorithm   string
		wantMetadataKey string
	}{
		// auth0 qs setup --app --type spa --framework react --build-tool vite
		{
			name:  "spa react vite",
			input: SetupInputs{Name: "React App", Port: 5173},
			reqParams: auth0.RequestParams{
				AppType:           "spa",
				Callbacks:         []string{sub},
				AllowedLogoutURLs: []string{sub},
				WebOrigins:        []string{sub},
				Name:              sub,
			},
			wantName:        "React App",
			wantAppType:     "spa",
			wantCallbacks:   []string{"http://localhost:5173/callback"},
			wantLogouts:     []string{"http://localhost:5173"},
			wantWebOrigins:  &[]string{"http://localhost:5173"},
			wantOIDC:        true,
			wantAlgorithm:   "RS256",
			wantMetadataKey: "created_by",
		},
		// auth0 qs setup --app --type spa --framework angular
		{
			name:  "spa angular no web-origins",
			input: SetupInputs{Name: "Angular App", Port: 4200},
			reqParams: auth0.RequestParams{
				AppType:           "spa",
				Callbacks:         []string{sub},
				AllowedLogoutURLs: []string{sub},
				Name:              sub,
				// No WebOrigins — angular doesn't need them
			},
			wantName:       "Angular App",
			wantAppType:    "spa",
			wantCallbacks:  []string{"http://localhost:4200/callback"},
			wantLogouts:    []string{"http://localhost:4200"},
			wantWebOrigins: nil,
			wantOIDC:       true,
			wantAlgorithm:  "RS256",
		},
		// auth0 qs setup --app --type regular --framework nextjs
		{
			name:  "regular nextjs",
			input: SetupInputs{Name: "Next App", Port: 3000},
			reqParams: auth0.RequestParams{
				AppType:           "regular_web",
				Callbacks:         []string{sub},
				AllowedLogoutURLs: []string{sub},
				Name:              sub,
			},
			wantName:       "Next App",
			wantAppType:    "regular_web",
			wantCallbacks:  []string{"http://localhost:3000/callback"},
			wantLogouts:    []string{"http://localhost:3000"},
			wantWebOrigins: nil,
			wantOIDC:       true,
			wantAlgorithm:  "RS256",
		},
		// auth0 qs setup --app --type native --framework flutter
		{
			name:  "native flutter",
			input: SetupInputs{Name: "Flutter App", Port: 3000},
			reqParams: auth0.RequestParams{
				AppType:           "native",
				Callbacks:         []string{sub},
				AllowedLogoutURLs: []string{sub},
				Name:              sub,
			},
			wantName:       "Flutter App",
			wantAppType:    "native",
			wantCallbacks:  []string{"http://localhost:3000/callback"},
			wantLogouts:    []string{"http://localhost:3000"},
			wantWebOrigins: nil,
			wantOIDC:       true,
			wantAlgorithm:  "RS256",
		},
		// auth0 qs setup --app --type regular --framework spring-boot (port 8080)
		{
			name:  "regular spring-boot port 8080",
			input: SetupInputs{Name: "Spring App", Port: 8080},
			reqParams: auth0.RequestParams{
				AppType:           "regular_web",
				Callbacks:         []string{sub},
				AllowedLogoutURLs: []string{sub},
				Name:              sub,
			},
			wantName:      "Spring App",
			wantCallbacks: []string{"http://localhost:8080/callback"},
			wantLogouts:   []string{"http://localhost:8080"},
			wantOIDC:      true,
			wantAlgorithm: "RS256",
		},
		// Name defaults to "My App" when empty
		{
			name:  "empty name defaults to My App",
			input: SetupInputs{Port: 3000},
			reqParams: auth0.RequestParams{
				AppType: "regular_web",
				Name:    sub,
			},
			wantName:      "My App",
			wantOIDC:      true,
			wantAlgorithm: "RS256",
		},
		// Port 0 defaults to 3000
		{
			name:  "port 0 defaults to 3000",
			input: SetupInputs{Name: "App", Port: 0},
			reqParams: auth0.RequestParams{
				AppType:   "regular_web",
				Callbacks: []string{sub},
				Name:      sub,
			},
			wantName:      "App",
			wantCallbacks: []string{"http://localhost:3000/callback"},
			wantOIDC:      true,
			wantAlgorithm: "RS256",
		},
		// Custom metadata is preserved (not overwritten by default)
		{
			name: "custom metadata preserved",
			input: SetupInputs{
				Name:     "App",
				Port:     3000,
				MetaData: map[string]interface{}{"env": "staging"},
			},
			reqParams: auth0.RequestParams{AppType: "spa"},
			wantName:  "App",
			wantOIDC:  true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client, err := generateClient(tc.input, tc.reqParams)
			require.NoError(t, err)
			require.NotNil(t, client)

			assert.Equal(t, tc.wantName, client.GetName())

			if tc.wantAppType != "" {
				assert.Equal(t, tc.wantAppType, client.GetAppType())
			}
			if tc.wantCallbacks != nil {
				assert.Equal(t, tc.wantCallbacks, client.GetCallbacks())
			}
			if tc.wantLogouts != nil {
				assert.Equal(t, tc.wantLogouts, client.GetAllowedLogoutURLs())
			}
			if tc.wantWebOrigins != nil {
				require.NotNil(t, client.WebOrigins)
				assert.Equal(t, *tc.wantWebOrigins, client.GetWebOrigins())
			} else {
				assert.Nil(t, client.WebOrigins)
			}
			if tc.wantOIDC {
				assert.True(t, client.GetOIDCConformant())
			}
			if tc.wantAlgorithm != "" {
				assert.Equal(t, tc.wantAlgorithm, client.GetJWTConfiguration().GetAlgorithm())
			}
			if tc.wantMetadataKey != "" {
				require.NotNil(t, client.ClientMetadata)
				assert.Contains(t, *client.ClientMetadata, tc.wantMetadataKey)
			}
		})
	}
}

func TestGenerateClient_DefaultMetadata(t *testing.T) {
	// When MetaData is nil in SetupInputs, generateClient must inject the default metadata.
	client, err := generateClient(
		SetupInputs{Name: "App", Port: 3000},
		auth0.RequestParams{AppType: "spa"},
	)
	require.NoError(t, err)
	require.NotNil(t, client.ClientMetadata)
	assert.Equal(t, "quickstart-docs-manual-cli", (*client.ClientMetadata)["created_by"])
}

func TestGenerateClient_CustomMetadataNotOverwritten(t *testing.T) {
	// When MetaData is provided in SetupInputs, it must NOT be replaced with the default.
	custom := map[string]interface{}{"source": "ci-pipeline"}
	client, err := generateClient(
		SetupInputs{Name: "App", Port: 3000, MetaData: custom},
		auth0.RequestParams{AppType: "regular_web"},
	)
	require.NoError(t, err)
	require.NotNil(t, client.ClientMetadata)
	assert.Equal(t, "ci-pipeline", (*client.ClientMetadata)["source"])
	assert.NotContains(t, *client.ClientMetadata, "created_by")
}

// ── getSupportedQuickstartTypes ───────────────────────────────────────────────

func TestGetSupportedQuickstartTypes(t *testing.T) {
	types := getSupportedQuickstartTypes()

	assert.NotEmpty(t, types)
	assert.True(t, sort.StringsAreSorted(types), "types should be sorted")

	// Spot-check representative keys from each app-type bucket.
	requiredKeys := []string{
		// SPA
		"spa:react:vite",
		"spa:angular:none",
		"spa:vue:vite",
		"spa:svelte:vite",
		"spa:vanilla-javascript:vite",
		"spa:flutter-web:none",
		// Regular
		"regular:nextjs:none",
		"regular:nuxt:none",
		"regular:fastify:none",
		"regular:sveltekit:none",
		"regular:express:none",
		"regular:hono:none",
		"regular:vanilla-python:none",
		"regular:django:none",
		"regular:vanilla-go:none",
		"regular:vanilla-java:maven",
		"regular:java-ee:maven",
		"regular:spring-boot:maven",
		"regular:aspnet-mvc:none",
		"regular:aspnet-blazor:none",
		"regular:aspnet-owin:none",
		"regular:vanilla-php:composer",
		"regular:laravel:composer",
		"regular:rails:none",
		// Native
		"native:flutter:none",
		"native:react-native:none",
		"native:expo:none",
		"native:ionic-angular:none",
		"native:ionic-react:vite",
		"native:ionic-vue:vite",
		"native:dotnet-mobile:none",
		"native:maui:none",
		"native:wpf-winforms:none",
	}

	for _, key := range requiredKeys {
		assert.Contains(t, types, key, "missing required key: %s", key)
	}
}

// ── setupQuickstartCmdExperimental – command-level interaction flows ───────────
//
// These tests exercise the RunE handler to verify the 7 top-level interaction
// flow paths. Because setupWithAuthentication reads ~/ config, we redirect
// HOME to a fresh temp dir so every test starts from a clean, unauthenticated
// state and gets a deterministic "config.json file is missing" error.

// flow 1: --app with all flags set (no interactive prompts needed)
func TestSetupQuickstartCmdExperimental_AppAllFlagsAuthRequired(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	cliObj := &cli{}
	cmd := setupQuickstartCmdExperimental(cliObj)
	cmd.SetArgs([]string{
		"--app",
		"--name", "My App",
		"--type", "spa",
		"--framework", "react",
		"--build-tool", "vite",
		"--port", "5173",
	})
	err := cmd.Execute()
	assert.EqualError(t, err, "authentication required: config.json file is missing")
}

// flow 2: --api only (no --app)
func TestSetupQuickstartCmdExperimental_APIOnlyAuthRequired(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	cliObj := &cli{}
	cmd := setupQuickstartCmdExperimental(cliObj)
	cmd.SetArgs([]string{
		"--api",
		"--identifier", "https://my-api.example.com",
		"--signing-alg", "RS256",
	})
	err := cmd.Execute()
	assert.EqualError(t, err, "authentication required: config.json file is missing")
}

// flow 3: --app and --api together (creates both resources)
func TestSetupQuickstartCmdExperimental_AppAndAPIAuthRequired(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	cliObj := &cli{}
	cmd := setupQuickstartCmdExperimental(cliObj)
	cmd.SetArgs([]string{
		"--app",
		"--name", "Express App",
		"--type", "regular",
		"--framework", "express",
		"--port", "3000",
		"--api",
		"--identifier", "https://example",
		"--signing-alg", "RS256",
	})
	err := cmd.Execute()
	assert.EqualError(t, err, "authentication required: config.json file is missing")
}

// flow 4: SPA frameworks – each framework/build-tool combo requires auth
func TestSetupQuickstartCmdExperimental_SPAFrameworks(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	spaTests := []struct {
		framework string
		buildTool string
		port      string
	}{
		{"react", "vite", "5173"},
		{"angular", "none", "4200"},
		{"vue", "vite", "5173"},
		{"svelte", "vite", "5173"},
		{"vanilla-javascript", "vite", "5173"},
		{"flutter-web", "none", "3000"},
	}

	for _, tc := range spaTests {
		t.Run(tc.framework, func(t *testing.T) {
			cliObj := &cli{}
			cmd := setupQuickstartCmdExperimental(cliObj)
			cmd.SetArgs([]string{
				"--app",
				"--name", tc.framework + "-app",
				"--type", "spa",
				"--framework", tc.framework,
				"--build-tool", tc.buildTool,
				"--port", tc.port,
			})
			err := cmd.Execute()
			assert.EqualError(t, err, "authentication required: config.json file is missing",
				"framework %s", tc.framework)
		})
	}
}

// flow 5: Regular web frameworks
func TestSetupQuickstartCmdExperimental_RegularFrameworks(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	regularTests := []struct {
		framework string
		buildTool string
		port      string
	}{
		{"nextjs", "none", "3000"},
		{"nuxt", "none", "3000"},
		{"fastify", "none", "3000"},
		{"sveltekit", "none", "3000"},
		{"express", "none", "3000"},
		{"hono", "none", "3000"},
		{"vanilla-python", "none", "5000"},
		{"django", "none", "3000"},
		{"vanilla-go", "none", "3000"},
		{"vanilla-java", "maven", "8080"},
		{"java-ee", "maven", "8080"},
		{"spring-boot", "maven", "8080"},
		{"aspnet-mvc", "none", "3000"},
		{"aspnet-blazor", "none", "3000"},
		{"aspnet-owin", "none", "3000"},
		{"vanilla-php", "composer", "3000"},
		{"laravel", "composer", "8000"},
		{"rails", "none", "3000"},
	}

	for _, tc := range regularTests {
		t.Run(tc.framework, func(t *testing.T) {
			cliObj := &cli{}
			cmd := setupQuickstartCmdExperimental(cliObj)
			cmd.SetArgs([]string{
				"--app",
				"--name", tc.framework + "-app",
				"--type", "regular",
				"--framework", tc.framework,
				"--build-tool", tc.buildTool,
				"--port", tc.port,
			})
			err := cmd.Execute()
			assert.EqualError(t, err, "authentication required: config.json file is missing",
				"framework %s", tc.framework)
		})
	}
}

// flow 6: Native / Mobile frameworks
func TestSetupQuickstartCmdExperimental_NativeFrameworks(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	nativeTests := []struct {
		framework string
		buildTool string
	}{
		{"flutter", "none"},
		{"react-native", "none"},
		{"expo", "none"},
		{"ionic-angular", "none"},
		{"ionic-react", "vite"},
		{"ionic-vue", "vite"},
		{"dotnet-mobile", "none"},
		{"maui", "none"},
		{"wpf-winforms", "none"},
	}

	for _, tc := range nativeTests {
		t.Run(tc.framework, func(t *testing.T) {
			cliObj := &cli{}
			cmd := setupQuickstartCmdExperimental(cliObj)
			cmd.SetArgs([]string{
				"--app",
				"--name", tc.framework + "-app",
				"--type", "native",
				"--framework", tc.framework,
				"--build-tool", tc.buildTool,
				"--port", "3000",
			})
			err := cmd.Execute()
			assert.EqualError(t, err, "authentication required: config.json file is missing",
				"framework %s", tc.framework)
		})
	}
}

// flow 7: auto-detection path – the command reads from CWD, which is controlled
// by the caller; with no auth config the command still fails at auth before
// attempting detection.
func TestSetupQuickstartCmdExperimental_DetectionPathAuthRequired(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	// Create a React project in a temp dir so detection would fire if auth passed.
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "vite.config.ts"), nil, 0600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "package.json"),
		[]byte(`{"name":"my-react-app","dependencies":{"react":"^18"}}`), 0600))

	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	cliObj := &cli{}
	cmd := setupQuickstartCmdExperimental(cliObj)
	cmd.SetArgs([]string{"--app"})
	err = cmd.Execute()
	assert.EqualError(t, err, "authentication required: config.json file is missing")
}

func TestGenerateAndWriteQuickstartConfig_NilStrategyDefaultsToDotenv(t *testing.T) {
	origDir, err := os.Getwd()
	require.NoError(t, err)
	dir := t.TempDir()
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	clientID := "cid"
	client := &management.Client{ClientID: &clientID}

	fileName, _, err := GenerateAndWriteQuickstartConfig(nil, map[string]string{"AUTH0_DOMAIN": "example.com"}, "tenant.auth0.com", client, 3000)
	require.NoError(t, err)
	assert.Equal(t, ".env", fileName)
}
