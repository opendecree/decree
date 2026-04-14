# Go Public Checklist

**Status:** Planning
**Started:** 2026-03-29

---

## Goal

Prepare the repo for public release on GitHub. Ensure it's clean, professional, and ready for community consumption.

## Must-Do (before flipping to public)

- [ ] **Scan for secrets** — scan git history for leaked credentials, API keys, tokens. Tools: `gitleaks`, `trufflehog`, or `git log --all -p | grep -i password`
- [ ] **LICENSE** — verify Apache 2.0 file exists and is correct
- [ ] **Clean git history** — squash noisy CI fix commits, review full history for anything embarrassing
- [ ] **README review** — final pass: links work, examples are accurate, positioning is clear
- [ ] **Module paths** — confirm `github.com/opendecree/decree` is the final org/repo name. Changing after people import is painful.
- [ ] **Remove GOPRIVATE** — once public, no need for GOPRIVATE/GONOSUMCHECK. pkg.go.dev will index automatically.
- [ ] **GitHub repo settings** — description, topics (`go`, `grpc`, `configuration`, `multi-tenant`, `schema-driven`), website URL (docs site)

## Should-Do (before or shortly after)

- [ ] **Tag v0.1.0** — first release. Triggers CI to build images, enables `go install`, pkg.go.dev indexing
- [ ] **GitHub Actions** — verify GITHUB_TOKEN has packages:write for ghcr.io image push
- [ ] **Branch protection** — require PR reviews, require CI to pass before merge to main
- [ ] **Issue templates** — `.github/ISSUE_TEMPLATE/bug_report.md` + `feature_request.md`
- [ ] **SECURITY.md** — vulnerability reporting instructions
- [ ] **Code of Conduct** — `CODE_OF_CONDUCT.md` (Contributor Covenant is standard)

## Nice-to-Have

- [ ] **README badges** — CI status, Go version, license, ghcr.io image
- [ ] **Social preview** — repo card image for link sharing
- [ ] **GitHub Discussions** — enable for Q&A and community
- [ ] **Example repo** — separate repo with runnable examples per use case
- [ ] **Blog post / announcement** — introduce DECREE, explain positioning

## Implementation Order

1. Scan for secrets (safety first)
2. License + module path confirmation
3. Clean git history (last chance before public)
4. README final review
5. GitHub repo settings
6. Flip to public
7. Tag v0.1.0
8. Verify pkg.go.dev, ghcr.io, CI
9. Issue templates, SECURITY.md, Code of Conduct
10. Badges, social preview, discussions
