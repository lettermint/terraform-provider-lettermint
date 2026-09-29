---
page_title: "Release process"
description: |-
  Prepare and verify a signed Lettermint Terraform provider release.
---

# Release process

Use a tag that starts with `v` and points to a commit on `main`. The release workflow runs CI before it creates the GitHub release.

Configure these repository values:

- Set `LETTERMINT_RELEASE_APP_ID` as a repository variable.
- Set `LETTERMINT_RELEASE_APP_PRIVATE_KEY` as a repository secret.
- Set `GPG_PRIVATE_KEY` and `PASSPHRASE` in the protected `release` environment.
- Set `DISCORD_RELEASE_WEBHOOK_URL` for release notifications.

Add the matching GPG public key to the Terraform Registry. Register the public `lettermint/terraform-provider-lettermint` repository before the first release.

The release workflow uses the shared GitHub App token. The changelog workflow gets the commit name and email from the returned `app-slug`. It does not contain a fixed bot name.

For the first release, publish and install a signed prerelease first. Confirm that Terraform verifies the signature. Publish `v0.1.0` only after this check succeeds.

The release verification workflow checks the archives, checksums, detached signature, protocol 6 manifest, and Registry installation. The changelog and Discord workflows run after GitHub publishes the release.
