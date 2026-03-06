# Firety Lint Rules

This document is generated from Firety's Go rule catalog. Treat the rule IDs and metadata here as the authoritative product surface for automation, CI, SARIF, and future integrations. Profile-aware rules may also drive selected-profile guidance in `firety skill lint --explain`, but those hints stay heuristic and conservative.

## Structure

### `skill.target-not-found`
- Default severity: `error`
- Title: Missing target path
- Description: Target path does not exist.
- Why it matters: Firety cannot lint a directory that is not present.
- What good looks like: The skill bundle should have a readable entry document, a clear title, and working local references.
- Improvement hint: Fix the structural issue first so later quality findings reflect the real document instead of a broken file layout.
- Profile-aware: no
- Can include line numbers: no
- Autofix: `none`
- Docs slug: `skill-target-not-found`

### `skill.target-not-directory`
- Default severity: `error`
- Title: Target is not a directory
- Description: Target path is not a directory.
- Why it matters: Firety only lints skill directories, not standalone files.
- What good looks like: The skill bundle should have a readable entry document, a clear title, and working local references.
- Improvement hint: Fix the structural issue first so later quality findings reflect the real document instead of a broken file layout.
- Profile-aware: no
- Can include line numbers: no
- Autofix: `none`
- Docs slug: `skill-target-not-directory`

### `skill.missing-skill-md`
- Default severity: `error`
- Title: Missing SKILL.md
- Description: SKILL.md is missing.
- Why it matters: A skill bundle needs a SKILL.md entrypoint for Firety to lint it.
- What good looks like: The skill bundle should have a readable entry document, a clear title, and working local references.
- Improvement hint: Fix the structural issue first so later quality findings reflect the real document instead of a broken file layout.
- Profile-aware: no
- Can include line numbers: no
- Autofix: `none`
- Docs slug: `skill-missing-skill-md`

### `skill.unreadable-skill-md`
- Default severity: `error`
- Title: Unreadable SKILL.md
- Description: SKILL.md cannot be read.
- Why it matters: Firety cannot inspect a skill definition if the entry document is unreadable.
- What good looks like: The skill bundle should have a readable entry document, a clear title, and working local references.
- Improvement hint: Fix the structural issue first so later quality findings reflect the real document instead of a broken file layout.
- Profile-aware: no
- Can include line numbers: no
- Autofix: `none`
- Docs slug: `skill-unreadable-skill-md`

### `skill.empty-skill-md`
- Default severity: `error`
- Title: Empty SKILL.md
- Description: SKILL.md is empty.
- Why it matters: An empty skill definition cannot communicate scope, usage, or examples.
- What good looks like: The skill bundle should have a readable entry document, a clear title, and working local references.
- Improvement hint: Fix the structural issue first so later quality findings reflect the real document instead of a broken file layout.
- Profile-aware: no
- Can include line numbers: no
- Autofix: `none`
- Docs slug: `skill-empty-skill-md`

### `skill.missing-title`
- Default severity: `error`
- Title: Missing top-level title
- Description: SKILL.md is missing a top-level markdown title.
- Why it matters: A missing title weakens readability and makes the skill harder to identify.
- What good looks like: The document should start with one clear `#` title that matches the skill identity used everywhere else.
- Improvement hint: Add a single top-level title near the start of `SKILL.md`, then keep the rest of the headings under it.
- Profile-aware: no
- Can include line numbers: yes
- Autofix: `automatic`
- Fix hint: Run `firety skill lint --fix` to insert a safe placeholder title, then edit it if needed.
- Docs slug: `skill-missing-title`

### `skill.broken-local-link`
- Default severity: `error`
- Title: Broken local link
- Description: Local markdown link points to a missing file.
- Why it matters: Broken links make a skill feel incomplete and can hide missing supporting material.
- What good looks like: The skill bundle should have a readable entry document, a clear title, and working local references.
- Improvement hint: Fix the structural issue first so later quality findings reflect the real document instead of a broken file layout.
- Profile-aware: no
- Can include line numbers: yes
- Autofix: `none`
- Docs slug: `skill-broken-local-link`

### `skill.large-content`
- Default severity: `warning`
- Title: Large markdown content
- Description: SKILL.md is very large and may be hard to maintain.
- Why it matters: Very large documents are harder to review and evolve cleanly.
- What good looks like: The skill bundle should have a readable entry document, a clear title, and working local references.
- Improvement hint: Fix the structural issue first so later quality findings reflect the real document instead of a broken file layout.
- Profile-aware: no
- Can include line numbers: no
- Autofix: `none`
- Docs slug: `skill-large-content`

### `skill.duplicate-heading`
- Default severity: `warning`
- Title: Duplicate heading
- Description: Duplicate heading found.
- Why it matters: Duplicate headings make the document harder to navigate and maintain.
- What good looks like: The skill bundle should have a readable entry document, a clear title, and working local references.
- Improvement hint: Fix the structural issue first so later quality findings reflect the real document instead of a broken file layout.
- Profile-aware: no
- Can include line numbers: yes
- Autofix: `none`
- Docs slug: `skill-duplicate-heading`

### `skill.suspicious-relative-path`
- Default severity: `warning`
- Title: Suspicious relative path
- Description: Relative path looks suspicious.
- Why it matters: Odd relative paths often signal portability or maintenance problems.
- What good looks like: The skill bundle should have a readable entry document, a clear title, and working local references.
- Improvement hint: Fix the structural issue first so later quality findings reflect the real document instead of a broken file layout.
- Profile-aware: no
- Can include line numbers: yes
- Autofix: `none`
- Docs slug: `skill-suspicious-relative-path`

### `skill.short-content`
- Default severity: `warning`
- Title: Very short content
- Description: SKILL.md content is very short and may not be useful.
- Why it matters: Very short skills rarely contain enough guidance to be trustworthy or reusable.
- What good looks like: The skill bundle should have a readable entry document, a clear title, and working local references.
- Improvement hint: Fix the structural issue first so later quality findings reflect the real document instead of a broken file layout.
- Profile-aware: no
- Can include line numbers: no
- Autofix: `none`
- Docs slug: `skill-short-content`

## Metadata / Spec

### `skill.invalid-front-matter`
- Default severity: `error`
- Title: Invalid front matter
- Description: SKILL.md front matter is malformed.
- Why it matters: Broken front matter makes metadata unreliable for humans and automation.
- What good looks like: Good metadata names the skill clearly and describes its purpose and boundaries in a compact, trustworthy way.
- Improvement hint: Tighten the front matter so the name and description make the skill easier to identify, route, and catalog.
- Profile-aware: no
- Can include line numbers: yes
- Autofix: `none`
- Docs slug: `skill-invalid-front-matter`

### `skill.missing-front-matter-name`
- Default severity: `error`
- Title: Missing front matter name
- Description: Front matter is missing a name field.
- Why it matters: A missing name weakens discoverability and downstream cataloging.
- What good looks like: Good metadata names the skill clearly and describes its purpose and boundaries in a compact, trustworthy way.
- Improvement hint: Tighten the front matter so the name and description make the skill easier to identify, route, and catalog.
- Profile-aware: no
- Can include line numbers: yes
- Autofix: `none`
- Docs slug: `skill-missing-front-matter-name`

### `skill.empty-front-matter-name`
- Default severity: `error`
- Title: Empty front matter name
- Description: Front matter name is empty.
- Why it matters: An empty name prevents the skill from being identified clearly.
- What good looks like: Good metadata names the skill clearly and describes its purpose and boundaries in a compact, trustworthy way.
- Improvement hint: Tighten the front matter so the name and description make the skill easier to identify, route, and catalog.
- Profile-aware: no
- Can include line numbers: yes
- Autofix: `none`
- Docs slug: `skill-empty-front-matter-name`

### `skill.missing-front-matter-description`
- Default severity: `warning`
- Strict severity: `error`
- Pedantic severity: `error`
- Title: Missing front matter description
- Description: Front matter is missing a description field.
- Why it matters: A missing description makes the skill harder to route and catalog.
- What good looks like: Good metadata names the skill clearly and describes its purpose and boundaries in a compact, trustworthy way.
- Improvement hint: Tighten the front matter so the name and description make the skill easier to identify, route, and catalog.
- Profile-aware: no
- Can include line numbers: yes
- Autofix: `none`
- Docs slug: `skill-missing-front-matter-description`

### `skill.empty-front-matter-description`
- Default severity: `warning`
- Strict severity: `error`
- Pedantic severity: `error`
- Title: Empty front matter description
- Description: Front matter description is empty.
- Why it matters: An empty description weakens routing, search, and documentation quality.
- What good looks like: Good metadata names the skill clearly and describes its purpose and boundaries in a compact, trustworthy way.
- Improvement hint: Tighten the front matter so the name and description make the skill easier to identify, route, and catalog.
- Profile-aware: no
- Can include line numbers: yes
- Autofix: `none`
- Docs slug: `skill-empty-front-matter-description`

### `skill.long-front-matter-name`
- Default severity: `warning`
- Title: Long front matter name
- Description: Front matter name is unusually long.
- Why it matters: Very long names are harder to scan in catalogs and selection UIs.
- What good looks like: Good metadata names the skill clearly and describes its purpose and boundaries in a compact, trustworthy way.
- Improvement hint: Tighten the front matter so the name and description make the skill easier to identify, route, and catalog.
- Profile-aware: no
- Can include line numbers: yes
- Autofix: `none`
- Docs slug: `skill-long-front-matter-name`

### `skill.short-front-matter-description`
- Default severity: `warning`
- Title: Short front matter description
- Description: Front matter description is too short to be useful.
- Why it matters: Very short descriptions rarely explain a skill's purpose or boundaries well.
- What good looks like: Good metadata names the skill clearly and describes its purpose and boundaries in a compact, trustworthy way.
- Improvement hint: Tighten the front matter so the name and description make the skill easier to identify, route, and catalog.
- Profile-aware: no
- Can include line numbers: yes
- Autofix: `none`
- Docs slug: `skill-short-front-matter-description`

### `skill.long-front-matter-description`
- Default severity: `warning`
- Title: Long front matter description
- Description: Front matter description is excessively long.
- Why it matters: Overlong descriptions often hide the core trigger and make metadata noisy.
- What good looks like: Good metadata names the skill clearly and describes its purpose and boundaries in a compact, trustworthy way.
- Improvement hint: Tighten the front matter so the name and description make the skill easier to identify, route, and catalog.
- Profile-aware: no
- Can include line numbers: yes
- Autofix: `none`
- Docs slug: `skill-long-front-matter-description`

### `skill.vague-description`
- Default severity: `warning`
- Title: Vague description
- Description: Description is too vague to clearly distinguish the skill.
- Why it matters: Generic descriptions make it harder for users and agents to know when this skill is the right fit.
- What good looks like: The description should quickly say what the skill does, when it should be used, and what makes it distinct.
- Improvement hint: Replace generic phrasing with one concrete purpose statement that names the trigger and the kind of outcome the skill is meant to produce.
- Profile-aware: no
- Can include line numbers: yes
- Autofix: `none`
- Docs slug: `skill-vague-description`

## Invocation

### `skill.missing-when-to-use`
- Default severity: `warning`
- Strict severity: `error`
- Pedantic severity: `error`
- Title: Missing when-to-use guidance
- Description: No obvious guidance explains when to use the skill.
- Why it matters: A skill should explain its intended trigger situations directly.
- What good looks like: A strong skill says when it should be selected, not just what it contains.
- Improvement hint: Add a short `When to use` section that names the user requests, task shapes, or situations that should trigger the skill.
- Profile-aware: no
- Can include line numbers: no
- Autofix: `none`
- Docs slug: `skill-missing-when-to-use`

### `skill.missing-usage-guidance`
- Default severity: `warning`
- Strict severity: `error`
- Pedantic severity: `error`
- Title: Missing usage guidance
- Description: No obvious usage or invocation guidance found.
- Why it matters: A skill should explain how to invoke it or what inputs it expects.
- What good looks like: A usable skill explains what information or request pattern the caller should provide.
- Improvement hint: Add a short usage section that states the expected inputs, invocation framing, or key parameters the skill needs to work well.
- Profile-aware: no
- Can include line numbers: no
- Autofix: `none`
- Docs slug: `skill-missing-usage-guidance`

## Examples

### `skill.missing-examples`
- Default severity: `warning`
- Pedantic severity: `error`
- Title: Missing examples
- Description: No obvious examples section found.
- Why it matters: Examples make a skill easier to understand and use correctly.
- What good looks like: At least one realistic example should show what kind of request triggers the skill and what a good result looks like.
- Improvement hint: Add a small examples section with one or two concrete requests, the expected invocation pattern, and the outcome or output style.
- Profile-aware: no
- Can include line numbers: no
- Autofix: `none`
- Docs slug: `skill-missing-examples`

### `skill.weak-examples`
- Default severity: `warning`
- Title: Weak examples
- Description: Examples exist but appear too short or content-light.
- Why it matters: Thin examples often fail to show a realistic usage pattern.
- What good looks like: Good examples show a realistic request, a concrete invocation pattern, and the shape of a useful result.
- Improvement hint: Revise the examples so they look concrete, complete, and aligned with the documented scope of the skill.
- Profile-aware: no
- Can include line numbers: yes
- Autofix: `none`
- Docs slug: `skill-weak-examples`

### `skill.generic-examples`
- Default severity: `warning`
- Title: Generic examples
- Description: Examples exist but appear too generic.
- Why it matters: Examples should make the skill feel concrete, not interchangeable.
- What good looks like: Good examples show a realistic request, a concrete invocation pattern, and the shape of a useful result.
- Improvement hint: Revise the examples so they look concrete, complete, and aligned with the documented scope of the skill.
- Profile-aware: no
- Can include line numbers: yes
- Autofix: `none`
- Docs slug: `skill-generic-examples`

### `skill.examples-missing-invocation-pattern`
- Default severity: `warning`
- Title: Examples missing invocation pattern
- Description: Examples do not show a clear invocation or request pattern.
- Why it matters: Examples should show how the skill gets triggered in practice.
- What good looks like: Good examples show a realistic request, a concrete invocation pattern, and the shape of a useful result.
- Improvement hint: Revise the examples so they look concrete, complete, and aligned with the documented scope of the skill.
- Profile-aware: no
- Can include line numbers: yes
- Autofix: `none`
- Docs slug: `skill-examples-missing-invocation-pattern`

### `skill.abstract-examples`
- Default severity: `warning`
- Title: Abstract examples
- Description: Examples are too abstract to be practically useful.
- Why it matters: Abstract examples rarely help a user or agent apply the skill correctly.
- What good looks like: Good examples show a realistic request, a concrete invocation pattern, and the shape of a useful result.
- Improvement hint: Revise the examples so they look concrete, complete, and aligned with the documented scope of the skill.
- Profile-aware: no
- Can include line numbers: yes
- Autofix: `none`
- Docs slug: `skill-abstract-examples`

### `skill.placeholder-heavy-examples`
- Default severity: `warning`
- Title: Placeholder-heavy examples
- Description: Examples rely too heavily on placeholders instead of concrete values.
- Why it matters: Too many placeholders make examples feel unfinished and hard to follow.
- What good looks like: Good examples show a realistic request, a concrete invocation pattern, and the shape of a useful result.
- Improvement hint: Revise the examples so they look concrete, complete, and aligned with the documented scope of the skill.
- Profile-aware: no
- Can include line numbers: yes
- Autofix: `none`
- Docs slug: `skill-placeholder-heavy-examples`

### `skill.examples-missing-expected-outcome`
- Default severity: `warning`
- Title: Examples missing expected outcome
- Description: Examples show a trigger but not the expected outcome or output style.
- Why it matters: Good examples usually show both how to start and what success looks like.
- What good looks like: Good examples show a realistic request, a concrete invocation pattern, and the shape of a useful result.
- Improvement hint: Revise the examples so they look concrete, complete, and aligned with the documented scope of the skill.
- Profile-aware: no
- Can include line numbers: yes
- Autofix: `none`
- Docs slug: `skill-examples-missing-expected-outcome`

### `skill.examples-missing-trigger-input`
- Default severity: `warning`
- Title: Examples missing trigger input
- Description: Examples show outcomes without a clear triggering input.
- Why it matters: Output-only examples make it harder to understand how to invoke the skill.
- What good looks like: Good examples show a realistic request, a concrete invocation pattern, and the shape of a useful result.
- Improvement hint: Revise the examples so they look concrete, complete, and aligned with the documented scope of the skill.
- Profile-aware: no
- Can include line numbers: yes
- Autofix: `none`
- Docs slug: `skill-examples-missing-trigger-input`

### `skill.example-scope-contradiction`
- Default severity: `warning`
- Title: Example scope contradiction
- Description: Examples appear to contradict the skill's documented scope or limitations.
- Why it matters: Examples should reinforce the documented scope rather than undermine it.
- What good looks like: Good examples show a realistic request, a concrete invocation pattern, and the shape of a useful result.
- Improvement hint: Revise the examples so they look concrete, complete, and aligned with the documented scope of the skill.
- Profile-aware: no
- Can include line numbers: yes
- Autofix: `none`
- Docs slug: `skill-example-scope-contradiction`

### `skill.example-guidance-mismatch`
- Default severity: `warning`
- Title: Example guidance mismatch
- Description: Examples appear inconsistent with the when-to-use guidance.
- Why it matters: If examples and guidance disagree, the skill's usage becomes ambiguous.
- What good looks like: Good examples show a realistic request, a concrete invocation pattern, and the shape of a useful result.
- Improvement hint: Revise the examples so they look concrete, complete, and aligned with the documented scope of the skill.
- Profile-aware: no
- Can include line numbers: yes
- Autofix: `none`
- Docs slug: `skill-example-guidance-mismatch`

### `skill.incomplete-example`
- Default severity: `warning`
- Title: Incomplete example
- Description: An example appears incomplete or abruptly truncated.
- Why it matters: Incomplete examples make the skill feel unfinished and unreliable.
- What good looks like: Good examples show a realistic request, a concrete invocation pattern, and the shape of a useful result.
- Improvement hint: Revise the examples so they look concrete, complete, and aligned with the documented scope of the skill.
- Profile-aware: no
- Can include line numbers: yes
- Autofix: `none`
- Docs slug: `skill-incomplete-example`

### `skill.example-missing-bundle-resource`
- Default severity: `warning`
- Title: Example missing bundle resource
- Description: An example references a local bundle resource that is missing.
- Why it matters: Examples should not rely on local files that are absent from the bundle.
- What good looks like: Good examples show a realistic request, a concrete invocation pattern, and the shape of a useful result.
- Improvement hint: Revise the examples so they look concrete, complete, and aligned with the documented scope of the skill.
- Profile-aware: no
- Can include line numbers: yes
- Autofix: `none`
- Docs slug: `skill-example-missing-bundle-resource`

### `skill.low-variety-examples`
- Default severity: `warning`
- Title: Low-variety examples
- Description: Examples do not demonstrate enough realistic variation.
- Why it matters: A small amount of realistic variety helps show the skill's intended use space.
- What good looks like: Good examples show a realistic request, a concrete invocation pattern, and the shape of a useful result.
- Improvement hint: Revise the examples so they look concrete, complete, and aligned with the documented scope of the skill.
- Profile-aware: no
- Can include line numbers: yes
- Autofix: `none`
- Docs slug: `skill-low-variety-examples`

## Negative Guidance

### `skill.missing-negative-guidance`
- Default severity: `warning`
- Pedantic severity: `error`
- Title: Missing negative guidance
- Description: No obvious guidance explains when not to use the skill.
- Why it matters: Boundary guidance prevents misuse and improves routing quality.
- What good looks like: A trustworthy skill explains both its fit and its boundaries so it does not compete with generic assistant behavior.
- Improvement hint: Add a short limitations or `When not to use` section that names out-of-scope work, preferred alternatives, or handoff cases.
- Profile-aware: no
- Can include line numbers: no
- Autofix: `none`
- Docs slug: `skill-missing-negative-guidance`

### `skill.weak-negative-guidance`
- Default severity: `warning`
- Title: Weak negative guidance
- Description: Negative guidance exists but appears too weak or generic.
- Why it matters: Weak boundaries leave too much ambiguity about when the skill does not fit.
- What good looks like: A strong skill explains where its boundaries are and when another approach should be used instead.
- Improvement hint: Add clear limits, out-of-scope cases, or handoff guidance so misuse is less likely.
- Profile-aware: no
- Can include line numbers: yes
- Autofix: `none`
- Docs slug: `skill-weak-negative-guidance`

## Consistency

### `skill.name-title-mismatch`
- Default severity: `warning`
- Title: Name and title mismatch
- Description: Front matter name appears inconsistent with the title.
- Why it matters: Conflicting identity signals make the skill feel sloppy and harder to trust.
- What good looks like: The metadata, title, and body should all describe the same skill identity, scope, and intended use.
- Improvement hint: Align the metadata and body so the skill presents one coherent purpose and boundary.
- Profile-aware: no
- Can include line numbers: yes
- Autofix: `none`
- Docs slug: `skill-name-title-mismatch`

### `skill.description-body-mismatch`
- Default severity: `warning`
- Title: Description and body mismatch
- Description: Front matter description appears inconsistent with the body.
- Why it matters: Conflicting metadata and body content undermine discoverability and routing.
- What good looks like: The metadata, title, and body should all describe the same skill identity, scope, and intended use.
- Improvement hint: Align the metadata and body so the skill presents one coherent purpose and boundary.
- Profile-aware: no
- Can include line numbers: yes
- Autofix: `none`
- Docs slug: `skill-description-body-mismatch`

### `skill.scope-mismatch`
- Default severity: `warning`
- Title: Scope mismatch
- Description: Front matter scope appears inconsistent with the body.
- Why it matters: A skill should present one clear scope across metadata and instructions.
- What good looks like: The metadata, title, and body should all describe the same skill identity, scope, and intended use.
- Improvement hint: Align the metadata and body so the skill presents one coherent purpose and boundary.
- Profile-aware: no
- Can include line numbers: yes
- Autofix: `none`
- Docs slug: `skill-scope-mismatch`

## Portability

### `skill.tool-specific-branding`
- Default severity: `warning`
- Title: Tool-specific branding
- Description: Instructions are strongly branded around a specific tool ecosystem.
- Why it matters: Heavy branding can make a skill feel less portable unless the targeting is intentional and clear.
- What good looks like: If the skill is truly portable, the wording should stay mostly tool-neutral. If it is targeted, the target should be explicit and consistent.
- Improvement hint: Either remove unnecessary tool branding or add clear audience and boundary language so the targeting looks intentional rather than accidental.
- Profile-aware: yes
- Can include line numbers: yes
- Autofix: `none`
- Docs slug: `skill-tool-specific-branding`

### `skill.profile-incompatible-guidance`
- Default severity: `warning`
- Title: Profile-incompatible guidance
- Description: Guidance appears incompatible with the selected portability profile.
- Why it matters: Conflicting profile guidance makes the skill harder to reuse safely across ecosystems.
- What good looks like: A portable skill either stays generic across tools or states its intended tool target and limits explicitly.
- Improvement hint: Remove accidental tool-specific assumptions or state the intended ecosystem and boundary language more clearly.
- Profile-aware: yes
- Can include line numbers: yes
- Autofix: `none`
- Docs slug: `skill-profile-incompatible-guidance`

### `skill.tool-specific-install-assumption`
- Default severity: `warning`
- Title: Tool-specific install assumption
- Description: Instructions assume a tool-specific install location or filesystem layout.
- Why it matters: Hard-coded ecosystem paths reduce portability and often surprise users.
- What good looks like: A portable skill either stays generic across tools or states its intended tool target and limits explicitly.
- Improvement hint: Remove accidental tool-specific assumptions or state the intended ecosystem and boundary language more clearly.
- Profile-aware: yes
- Can include line numbers: yes
- Autofix: `none`
- Docs slug: `skill-tool-specific-install-assumption`

### `skill.nonportable-invocation-guidance`
- Default severity: `warning`
- Title: Nonportable invocation guidance
- Description: Invocation guidance depends on tool-specific commands or UX conventions.
- Why it matters: Tool-specific invocation language can accidentally lock a skill to one ecosystem.
- What good looks like: A portable skill either stays generic across tools or states its intended tool target and limits explicitly.
- Improvement hint: Remove accidental tool-specific assumptions or state the intended ecosystem and boundary language more clearly.
- Profile-aware: yes
- Can include line numbers: yes
- Autofix: `none`
- Docs slug: `skill-nonportable-invocation-guidance`

### `skill.generic-profile-tool-locking`
- Default severity: `warning`
- Title: Generic profile tool locking
- Description: The skill appears too tightly coupled to one tool for the generic profile.
- Why it matters: The generic profile should stay reserved for skills that read as broadly portable.
- What good looks like: A portable skill either stays generic across tools or states its intended tool target and limits explicitly.
- Improvement hint: Remove accidental tool-specific assumptions or state the intended ecosystem and boundary language more clearly.
- Profile-aware: yes
- Can include line numbers: yes
- Autofix: `none`
- Docs slug: `skill-generic-profile-tool-locking`

### `skill.unclear-tool-targeting`
- Default severity: `warning`
- Title: Unclear tool targeting
- Description: The skill uses tool-specific conventions without clearly stating its intended target.
- Why it matters: If a skill is targeted, it should say so explicitly instead of surprising the reader later.
- What good looks like: A portable skill either stays generic across tools or states its intended tool target and limits explicitly.
- Improvement hint: Remove accidental tool-specific assumptions or state the intended ecosystem and boundary language more clearly.
- Profile-aware: yes
- Can include line numbers: yes
- Autofix: `none`
- Docs slug: `skill-unclear-tool-targeting`

### `skill.accidental-tool-lock-in`
- Default severity: `warning`
- Title: Accidental tool lock-in
- Description: The skill appears unintentionally locked to one tool ecosystem.
- Why it matters: Accidental lock-in is usually worse than honest targeting because it is harder to reason about.
- What good looks like: A targeted skill should say so clearly; a portable skill should avoid depending on one ecosystem's commands, paths, or UX metaphors.
- Improvement hint: Decide whether the skill is truly generic or intentionally targeted, then make the wording, examples, and boundary guidance match that choice.
- Profile-aware: yes
- Can include line numbers: yes
- Autofix: `none`
- Docs slug: `skill-accidental-tool-lock-in`

### `skill.generic-portability-contradiction`
- Default severity: `warning`
- Strict severity: `error`
- Pedantic severity: `error`
- Title: Generic portability contradiction
- Description: The skill claims to be generic or portable but behaves as tool-specific.
- Why it matters: Contradictory portability claims make the skill hard to trust in automation and CI.
- What good looks like: A skill's stated portability posture should match its actual instructions, examples, and install assumptions.
- Improvement hint: Either make the wording and examples more tool-neutral or explicitly narrow the skill to the ecosystem it really targets.
- Profile-aware: yes
- Can include line numbers: yes
- Autofix: `none`
- Docs slug: `skill-generic-portability-contradiction`

### `skill.mixed-ecosystem-guidance`
- Default severity: `warning`
- Title: Mixed ecosystem guidance
- Description: Guidance mixes multiple tool ecosystems in a confusing way.
- Why it matters: Mixed instructions make it unclear which runtime or workflow the skill actually targets.
- What good looks like: A portable skill either stays generic across tools or states its intended tool target and limits explicitly.
- Improvement hint: Remove accidental tool-specific assumptions or state the intended ecosystem and boundary language more clearly.
- Profile-aware: yes
- Can include line numbers: yes
- Autofix: `none`
- Docs slug: `skill-mixed-ecosystem-guidance`

### `skill.missing-tool-target-boundary`
- Default severity: `warning`
- Title: Missing tool-target boundary
- Description: A tool-specific skill lacks explicit boundary guidance about its intended audience.
- Why it matters: Honest tool targeting should also explain who the skill is and is not for.
- What good looks like: A portable skill either stays generic across tools or states its intended tool target and limits explicitly.
- Improvement hint: Remove accidental tool-specific assumptions or state the intended ecosystem and boundary language more clearly.
- Profile-aware: yes
- Can include line numbers: yes
- Autofix: `none`
- Docs slug: `skill-missing-tool-target-boundary`

### `skill.profile-target-mismatch`
- Default severity: `warning`
- Strict severity: `error`
- Pedantic severity: `error`
- Title: Profile target mismatch
- Description: The selected profile conflicts with the skill's apparent intended target.
- Why it matters: A strong profile mismatch suggests the skill is being evaluated under the wrong portability posture.
- What good looks like: A portable skill either stays generic across tools or states its intended tool target and limits explicitly.
- Improvement hint: Remove accidental tool-specific assumptions or state the intended ecosystem and boundary language more clearly.
- Profile-aware: yes
- Can include line numbers: yes
- Autofix: `none`
- Docs slug: `skill-profile-target-mismatch`

### `skill.example-ecosystem-mismatch`
- Default severity: `warning`
- Title: Example ecosystem mismatch
- Description: Examples reinforce a different tool ecosystem than the rest of the skill.
- Why it matters: Examples should support the same portability posture as the rest of the document.
- What good looks like: A portable skill either stays generic across tools or states its intended tool target and limits explicitly.
- Improvement hint: Remove accidental tool-specific assumptions or state the intended ecosystem and boundary language more clearly.
- Profile-aware: yes
- Can include line numbers: yes
- Autofix: `none`
- Docs slug: `skill-example-ecosystem-mismatch`

## Bundle / Resources

### `skill.reference-outside-root`
- Default severity: `warning`
- Title: Reference outside skill root
- Description: Referenced resource escapes the skill root.
- Why it matters: References outside the bundle are brittle and reduce portability.
- What good looks like: A healthy skill bundle keeps referenced local resources present, useful, and inside the skill root.
- Improvement hint: Align the documented local resources with the actual bundle contents and remove brittle or misleading references.
- Profile-aware: no
- Can include line numbers: yes
- Autofix: `none`
- Docs slug: `skill-reference-outside-root`

### `skill.referenced-directory-instead-of-file`
- Default severity: `warning`
- Title: Directory referenced as a file
- Description: Referenced resource is a directory where a file is expected.
- Why it matters: A directory link often means the bundle structure or documentation is unclear.
- What good looks like: A healthy skill bundle keeps referenced local resources present, useful, and inside the skill root.
- Improvement hint: Align the documented local resources with the actual bundle contents and remove brittle or misleading references.
- Profile-aware: no
- Can include line numbers: yes
- Autofix: `none`
- Docs slug: `skill-referenced-directory-instead-of-file`

### `skill.empty-referenced-resource`
- Default severity: `warning`
- Title: Empty referenced resource
- Description: Referenced resource exists but is empty.
- Why it matters: An empty supporting file is usually accidental and not helpful to consumers.
- What good looks like: A healthy skill bundle keeps referenced local resources present, useful, and inside the skill root.
- Improvement hint: Align the documented local resources with the actual bundle contents and remove brittle or misleading references.
- Profile-aware: no
- Can include line numbers: yes
- Autofix: `none`
- Docs slug: `skill-empty-referenced-resource`

### `skill.suspicious-referenced-resource`
- Default severity: `warning`
- Title: Suspicious referenced resource
- Description: Referenced resource type looks suspicious for a skill bundle.
- Why it matters: Unexpected binary-style resources can make a bundle harder to trust or reuse.
- What good looks like: A healthy skill bundle keeps referenced local resources present, useful, and inside the skill root.
- Improvement hint: Align the documented local resources with the actual bundle contents and remove brittle or misleading references.
- Profile-aware: no
- Can include line numbers: yes
- Autofix: `none`
- Docs slug: `skill-suspicious-referenced-resource`

### `skill.duplicate-resource-reference`
- Default severity: `warning`
- Title: Duplicate resource reference
- Description: The same local resource is referenced repeatedly.
- Why it matters: Repeated references often signal clutter or accidental duplication in the document.
- What good looks like: A healthy skill bundle keeps referenced local resources present, useful, and inside the skill root.
- Improvement hint: Align the documented local resources with the actual bundle contents and remove brittle or misleading references.
- Profile-aware: no
- Can include line numbers: yes
- Autofix: `none`
- Docs slug: `skill-duplicate-resource-reference`

### `skill.missing-mentioned-resource`
- Default severity: `warning`
- Title: Missing mentioned resource
- Description: A strongly-mentioned local resource is missing from the bundle.
- Why it matters: If SKILL.md names a local helper, it should generally exist in the package.
- What good looks like: A healthy skill bundle keeps referenced local resources present, useful, and inside the skill root.
- Improvement hint: Align the documented local resources with the actual bundle contents and remove brittle or misleading references.
- Profile-aware: no
- Can include line numbers: yes
- Autofix: `none`
- Docs slug: `skill-missing-mentioned-resource`

### `skill.inconsistent-bundle-structure`
- Default severity: `warning`
- Title: Inconsistent bundle structure
- Description: The skill bundle structure appears inconsistent with the documentation.
- Why it matters: A bundle should match what the skill says is available.
- What good looks like: A healthy skill bundle keeps referenced local resources present, useful, and inside the skill root.
- Improvement hint: Align the documented local resources with the actual bundle contents and remove brittle or misleading references.
- Profile-aware: no
- Can include line numbers: no
- Autofix: `none`
- Docs slug: `skill-inconsistent-bundle-structure`

### `skill.possibly-stale-resource`
- Default severity: `warning`
- Title: Possibly stale resource
- Description: The bundle contains helper resources that may be stale or unused.
- Why it matters: Stale bundle contents add maintenance cost and reduce trust in the package.
- What good looks like: A healthy skill bundle keeps referenced local resources present, useful, and inside the skill root.
- Improvement hint: Align the documented local resources with the actual bundle contents and remove brittle or misleading references.
- Profile-aware: no
- Can include line numbers: no
- Autofix: `none`
- Docs slug: `skill-possibly-stale-resource`

### `skill.unhelpful-referenced-resource`
- Default severity: `warning`
- Title: Unhelpful referenced resource
- Description: Referenced resource exists but appears too short to be useful.
- Why it matters: A referenced helper should provide enough substance to justify including it.
- What good looks like: A healthy skill bundle keeps referenced local resources present, useful, and inside the skill root.
- Improvement hint: Align the documented local resources with the actual bundle contents and remove brittle or misleading references.
- Profile-aware: no
- Can include line numbers: yes
- Autofix: `none`
- Docs slug: `skill-unhelpful-referenced-resource`

## Efficiency / Cost

### `skill.large-skill-md`
- Default severity: `warning`
- Title: Large SKILL.md
- Description: SKILL.md appears large enough to create unnecessary context cost.
- Why it matters: Large instructions increase maintenance cost and likely token usage.
- What good looks like: The main skill document should stay focused enough that an agent can load it without carrying a large amount of avoidable context.
- Improvement hint: Trim repeated prose, move optional detail into focused supporting resources, and keep `SKILL.md` centered on the core trigger, guidance, and examples.
- Profile-aware: no
- Can include line numbers: no
- Autofix: `none`
- Docs slug: `skill-large-skill-md`

### `skill.excessive-example-volume`
- Default severity: `warning`
- Title: Excessive example volume
- Description: Examples appear unusually large for a single skill.
- Why it matters: Too many example tokens can crowd out the core instructions.
- What good looks like: A well-sized skill keeps the core instructions and examples focused enough to stay cheap to load and easy to maintain.
- Improvement hint: Trim repeated or oversized content so the skill stays more focused, cheaper to load, and easier to evolve.
- Profile-aware: no
- Can include line numbers: yes
- Autofix: `none`
- Docs slug: `skill-excessive-example-volume`

### `skill.duplicate-examples`
- Default severity: `warning`
- Title: Duplicate examples
- Description: Examples appear duplicated or near-duplicated.
- Why it matters: Repeated examples add cost without improving understanding.
- What good looks like: A well-sized skill keeps the core instructions and examples focused enough to stay cheap to load and easy to maintain.
- Improvement hint: Trim repeated or oversized content so the skill stays more focused, cheaper to load, and easier to evolve.
- Profile-aware: no
- Can include line numbers: yes
- Autofix: `none`
- Docs slug: `skill-duplicate-examples`

### `skill.large-referenced-resource`
- Default severity: `warning`
- Title: Large referenced resource
- Description: A referenced resource appears large enough to be costly to load.
- Why it matters: Very large referenced text files can make a skill expensive to use.
- What good looks like: A well-sized skill keeps the core instructions and examples focused enough to stay cheap to load and easy to maintain.
- Improvement hint: Trim repeated or oversized content so the skill stays more focused, cheaper to load, and easier to evolve.
- Profile-aware: no
- Can include line numbers: yes
- Autofix: `none`
- Docs slug: `skill-large-referenced-resource`

### `skill.excessive-bundle-size`
- Default severity: `warning`
- Title: Excessive bundle size
- Description: The likely-loaded skill bundle appears excessively large.
- Why it matters: A single skill should usually stay compact enough to load cheaply.
- What good looks like: A well-sized skill keeps the core instructions and examples focused enough to stay cheap to load and easy to maintain.
- Improvement hint: Trim repeated or oversized content so the skill stays more focused, cheaper to load, and easier to evolve.
- Profile-aware: no
- Can include line numbers: no
- Autofix: `none`
- Docs slug: `skill-excessive-bundle-size`

### `skill.repetitive-instructions`
- Default severity: `warning`
- Title: Repetitive instructions
- Description: Instruction sections appear overly repetitive.
- Why it matters: Repeated instruction text increases cost without improving clarity.
- What good looks like: A well-sized skill keeps the core instructions and examples focused enough to stay cheap to load and easy to maintain.
- Improvement hint: Trim repeated or oversized content so the skill stays more focused, cheaper to load, and easier to evolve.
- Profile-aware: no
- Can include line numbers: yes
- Autofix: `none`
- Docs slug: `skill-repetitive-instructions`

### `skill.unbalanced-skill-content`
- Default severity: `warning`
- Title: Unbalanced skill content
- Description: The skill appears unbalanced between instructions and examples.
- Why it matters: Extreme imbalance can make a skill either vague or unnecessarily expensive.
- What good looks like: A well-sized skill keeps the core instructions and examples focused enough to stay cheap to load and easy to maintain.
- Improvement hint: Trim repeated or oversized content so the skill stays more focused, cheaper to load, and easier to evolve.
- Profile-aware: no
- Can include line numbers: yes
- Autofix: `none`
- Docs slug: `skill-unbalanced-skill-content`

## Trigger Quality

### `skill.generic-name`
- Default severity: `warning`
- Title: Generic skill name
- Description: Skill name is too generic to be distinctive.
- Why it matters: A generic name weakens discoverability and trigger quality.
- What good looks like: A strong skill name is short, specific, and distinctive enough that a user or agent can tell what it is for at a glance.
- Improvement hint: Rename the skill toward the concrete task or domain it handles instead of using broad labels like helper, assistant, or tool.
- Profile-aware: no
- Can include line numbers: yes
- Autofix: `none`
- Docs slug: `skill-generic-name`

### `skill.generic-trigger-description`
- Default severity: `warning`
- Title: Generic trigger description
- Description: Trigger description is too generic to distinguish the skill.
- Why it matters: If the description sounds like a general assistant, routing becomes less reliable.
- What good looks like: A high-quality trigger surface gives the skill a distinctive name, a clear trigger concept, and consistent routing signals.
- Improvement hint: Sharpen the name, description, and trigger guidance so the skill is easier to select at the right time.
- Profile-aware: no
- Can include line numbers: yes
- Autofix: `none`
- Docs slug: `skill-generic-trigger-description`

### `skill.diffuse-scope`
- Default severity: `warning`
- Title: Diffuse scope
- Description: Skill scope appears too broad or diffuse.
- Why it matters: Skills with scattered scope are harder to trigger at the right time.
- What good looks like: A high-quality trigger surface gives the skill a distinctive name, a clear trigger concept, and consistent routing signals.
- Improvement hint: Sharpen the name, description, and trigger guidance so the skill is easier to select at the right time.
- Profile-aware: no
- Can include line numbers: yes
- Autofix: `none`
- Docs slug: `skill-diffuse-scope`

### `skill.overbroad-when-to-use`
- Default severity: `warning`
- Title: Overbroad when-to-use guidance
- Description: When-to-use guidance appears too broad to route clearly.
- Why it matters: Overbroad guidance makes the skill compete with generic assistant behavior.
- What good looks like: A high-quality trigger surface gives the skill a distinctive name, a clear trigger concept, and consistent routing signals.
- Improvement hint: Sharpen the name, description, and trigger guidance so the skill is easier to select at the right time.
- Profile-aware: no
- Can include line numbers: yes
- Autofix: `none`
- Docs slug: `skill-overbroad-when-to-use`

### `skill.weak-trigger-pattern`
- Default severity: `warning`
- Title: Weak trigger pattern
- Description: Examples do not reinforce a clear trigger pattern.
- Why it matters: Examples should help reinforce when this skill should be selected.
- What good looks like: A high-quality trigger surface gives the skill a distinctive name, a clear trigger concept, and consistent routing signals.
- Improvement hint: Sharpen the name, description, and trigger guidance so the skill is easier to select at the right time.
- Profile-aware: no
- Can include line numbers: yes
- Autofix: `none`
- Docs slug: `skill-weak-trigger-pattern`

### `skill.low-distinctiveness`
- Default severity: `warning`
- Title: Low distinctiveness
- Description: The skill lacks distinctive trigger terms or phrases.
- Why it matters: Distinctive language helps the skill stand out from neighboring capabilities.
- What good looks like: A high-quality trigger surface gives the skill a distinctive name, a clear trigger concept, and consistent routing signals.
- Improvement hint: Sharpen the name, description, and trigger guidance so the skill is easier to select at the right time.
- Profile-aware: no
- Can include line numbers: yes
- Autofix: `none`
- Docs slug: `skill-low-distinctiveness`

### `skill.example-trigger-mismatch`
- Default severity: `warning`
- Title: Example trigger mismatch
- Description: Examples appear misaligned with the documented trigger concept.
- Why it matters: Misaligned examples weaken trust in the skill's stated purpose.
- What good looks like: A high-quality trigger surface gives the skill a distinctive name, a clear trigger concept, and consistent routing signals.
- Improvement hint: Sharpen the name, description, and trigger guidance so the skill is easier to select at the right time.
- Profile-aware: no
- Can include line numbers: yes
- Autofix: `none`
- Docs slug: `skill-example-trigger-mismatch`

### `skill.trigger-scope-inconsistency`
- Default severity: `warning`
- Title: Trigger scope inconsistency
- Description: Name, description, and body point to different trigger concepts.
- Why it matters: Trigger quality depends on one coherent concept across the whole document.
- What good looks like: A high-quality trigger surface gives the skill a distinctive name, a clear trigger concept, and consistent routing signals.
- Improvement hint: Sharpen the name, description, and trigger guidance so the skill is easier to select at the right time.
- Profile-aware: no
- Can include line numbers: yes
- Autofix: `none`
- Docs slug: `skill-trigger-scope-inconsistency`
