# ALB to Gateway API migration docs

Read these local files in order:

Before running the generator, read `../README.md`, `../SKILL.md`, this file,
`migrate-alb-to-gatewayapi.md`, `mappings.md`, and `inputs-and-export.md`.

1. `migrate-alb-to-gatewayapi.md` — main migration guide, targets ALB 4.3 and Envoy Gateway.
2. `mappings.md` — ALB fields, Gateway API targets, check IDs, limitations.
3. `inputs-and-export.md` — what to export, how to keep object relationships, privacy.
4. `review-and-cutover.md` — review checklist, validation, cutover, rollback.
5. `privacy.md` — data handling and public-sharing rules.

The guide is self-contained. Do not require external product-document links to
complete a migration.
