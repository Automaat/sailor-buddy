# Security scanning

The `Security Scan` CI job checks the complete frontend lockfile, including
development, optional and peer dependencies. Svelte, SvelteKit and adapter-node
are declared as development dependencies but contribute to the shipped app;
their declaration does not exempt them from scanning. `govulncheck -show verbose
./...` checks the backend and records package/module findings alongside reachable
symbol findings. Reports are retained as the `dependency-security` workflow
artifact for 90 days, including when a gate fails.

## Policy

- npm: high and critical findings block. Every lower-severity finding also blocks
  until reviewed. `.github/security-dispositions.json` permits only the recorded
  advisory URLs, dependency paths, parent relationships and installed versions.
  Each disposition names an owner, evidence, review date and deadline within
  31 days. Expired dispositions and changed exposure block CI. Remove records
  after the finding disappears; do not extend a deadline without a new review.
- Go: reachable vulnerable symbols block through govulncheck's exit status.
  Package/module findings remain in the verbose report for review; they do not
  prove that the vulnerable code is used.
- Images: Trivy scans all severities, both OS packages and recognizable language
  packages, without ignoring unfixed vulnerabilities. High and critical findings
  block; lower and unknown findings remain visible in the report and require
  review before deployment. There are no image vulnerability exceptions.
- Scanner errors, malformed reports, missing OS inventory, an unsupported OS, or
  a report for a different image fail the gate. A missing report is not clean.

The Docker Build workflow loads and scans each final image by its immutable image
ID. Docker Publish scans each pushed image by the exact GHCR `@sha256:...` digest
returned by BuildKit. It creates the GitHub release only after both scans pass,
and attaches each image's full JSON report and metadata. Metadata records source
commit, collection time, architecture, scanner/database version and image
identity. Workflow reports are also retained for 90 days when scanning fails.

Publication pushes image tags before scanning. A tag's existence is not release
approval: deploy only digests from a completed release with passing reports.
If a scan fails, investigate the report, repair and rebuild; do not deploy the
pushed image or bypass the gate. Builds pull current base images. If cached APK
layers retain a newly vulnerable package, rebuild without the affected cache.

Image scanning cannot identify every library bundled into JavaScript. The full
lockfile audit covers the frontend dependency graph, and bundle inspection
supports each build-only disposition. The final frontend image runs `node build`
without an application `node_modules` directory. Its unused npm installation is
removed; the build stage retains npm. Both final images apply Alpine updates.
Local scans on arm64 supplement the CI scans on amd64; a local pass does not
replace the release digest report. There is no deployed target yet; production
configuration and release evidence must still be verified in #690.

## Reviewed findings

Reviewed 2026-11-14 by Automaat; npm dispositions expire 2026-12-14.
The complete npm scan reported two advisories across nine package entries
(two low, seven moderate; zero high or critical). These are current observations,
not permanent expected counts.

| Advisory | Exposure and disposition |
| --- | --- |
| [GHSA-gcqx-w48m-w2h4](https://github.com/advisories/GHSA-gcqx-w48m-w2h4) | `deepmerge@4.3.1` is required by `@sveltejs/vite-plugin-svelte@7.3.1`. The plugin's `src/utils/options.js` merges Vite/Svelte configuration at build time. No patched deepmerge version is available. Accept the recorded build-tool scope until the deadline; builds consume repository-controlled configuration. |
| [GHSA-g3hv-75wg-m249](https://github.com/advisories/GHSA-g3hv-75wg-m249) | `minimatch@10.2.6` is nested under `@typescript-eslint/typescript-estree@8.72.2`, whose declared range is `^10.2.2`. `dist/useProgramFromProjectService.js` uses it for lint project-file globs. The fix starts at 11.0.1, outside that range. Accept only this lint-tool path until the deadline; do not force an incompatible override or npm's proposed typescript-eslint downgrade. The separate root minimatch 11.0.8 is patched. |

Evidence: `npm explain minimatch deepmerge`, the parent source paths above, and
inspection of the built final frontend image. The image has no `/app/node_modules`;
neither package name appears in `/app/build`, including generated source maps.
This string check supports the inspected import paths; it is not a general proof
that bundled dependencies are absent. Application imports or build configuration
changes require a new exposure review even if lockfile paths stay unchanged.
Dependabot alerts #19 and #22 track upstream fixes. Owner: Automaat; recheck both
at the deadline or when updating either dependency chain, whichever comes first.

The initial image scan also found `CVE-2026-4738` in zlib 1.3.2-r1 in both base
images, and high findings in npm's bundled brace-expansion and undici in the
frontend base image. Alpine updates install zlib 1.4-r0; removing the unused npm
tree removes those npm libraries from the final image. Rebuilt images must pass
the same scanner without exclusions.

Rebuilt arm64 images scanned with Trivy 0.76.2 on 2026-11-14 had zero OS findings
in both images and zero frontend language-package findings. The backend retained
only the three UNKNOWN module findings described below. Both image gates passed;
CI and release artifacts record the corresponding amd64 results and identities.

`GO-2026-5932` concerns `golang.org/x/crypto/openpgp`, which is not imported by the
backend. The 2026-11-14 govulncheck run found zero affected symbols, zero imported
package findings and this one required-module finding. Trivy also reports the
module in three backend binaries with UNKNOWN severity. Owner Automaat reviews
the verbose Go and image reports before release; do not infer reachability from
module presence or suppress the advisory across the module.

## Run locally

Use the repository's pinned tools and Python 3. Trivy's CI version is explicit in
`.github/actions/scan-image/action.yml`; install that version without changing
the repository's managed tool configuration.

```sh
mise exec -- python3 .github/scripts/security_scan.py npm --output /tmp/sailor-npm-report
cd backend
mise exec -- govulncheck -show verbose ./...
```

From the repository root, build a final image, obtain its ID with
`docker image inspect --format '{{.Id}}' IMAGE`, then run:

```sh
python3 .github/scripts/security_scan.py image \
  --image sha256:IMAGE_ID --source docker --output /tmp/sailor-image-report
python3 .github/scripts/test_security_scan.py
```

Use a real 64-character digest. For Docker contexts outside the default socket,
set Trivy's `TRIVY_DOCKER_HOST` to the context's Docker endpoint. To inspect a
release image, use `--source remote --image ghcr.io/OWNER/IMAGE@sha256:DIGEST` with
registry read access. Preserve the raw report and metadata together.
