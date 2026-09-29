package release

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func repositoryFile(t *testing.T, relativePath string) string {
	t.Helper()
	contents, err := os.ReadFile(filepath.Join("..", "..", relativePath))
	if err != nil {
		t.Fatalf("read %s: %v", relativePath, err)
	}
	return string(contents)
}

func requireText(t *testing.T, contents, text, file string) {
	t.Helper()
	if !strings.Contains(contents, text) {
		t.Errorf("%s does not contain %q", file, text)
	}
}

func TestReleaseContract(t *testing.T) {
	t.Parallel()

	goreleaser := repositoryFile(t, ".goreleaser.yml")
	for _, required := range []string{
		"formats:\n      - zip",
		"terraform-registry-manifest.json",
		"name_template: '{{ .ProjectName }}_{{ .Version }}_SHA256SUMS'",
		"artifacts: checksum",
		"'{{ .Env.GPG_FINGERPRINT }}'",
	} {
		requireText(t, goreleaser, required, ".goreleaser.yml")
	}

	releaseWorkflow := repositoryFile(t, ".github/workflows/release.yml")
	for _, required := range []string{
		"LETTERMINT_RELEASE_APP_ID",
		"LETTERMINT_RELEASE_APP_PRIVATE_KEY",
		"steps.release-bot.outputs.token",
		"environment: release",
		"GPG_PRIVATE_KEY",
		"PASSPHRASE",
		"release --clean",
	} {
		requireText(t, releaseWorkflow, required, ".github/workflows/release.yml")
	}

	verificationWorkflow := repositoryFile(t, ".github/workflows/verify-release.yml")
	for _, required := range []string{
		"gpg --verify",
		"sha256sum --check",
		"protocol_versions == [\"6.0\"]",
		"unzip -Z1",
		"terraform -chdir=registry-test init -backend=false",
	} {
		requireText(t, verificationWorkflow, required, ".github/workflows/verify-release.yml")
	}

	changelogWorkflow := repositoryFile(t, ".github/workflows/update-changelog.yml")
	for _, required := range []string{
		"steps.release-bot.outputs.app-slug",
		"bot_login=\"${RELEASE_BOT_SLUG}[bot]\"",
		"gh api \"users/${bot_login}\" --jq .id",
	} {
		requireText(t, changelogWorkflow, required, ".github/workflows/update-changelog.yml")
	}

	discordWorkflow := repositoryFile(t, ".github/workflows/release-to-discord.yml")
	requireText(t, discordWorkflow, "lettermint/action-discord-releases@v1.0.1", ".github/workflows/release-to-discord.yml")
	requireText(t, discordWorkflow, "DISCORD_RELEASE_WEBHOOK_URL", ".github/workflows/release-to-discord.yml")
}
