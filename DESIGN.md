# PanDrive Design Direction

## Identity

PanDrive is a practical multi-account Drive control room. It should feel like a reliable file manager, not a marketing site or a developer terminal.

## Audience

People managing large files, multiple Google Drive accounts, split uploads, transfers, and quota. They need operational clarity before visual novelty.

## Visual language

Keep the existing PanDrive interface as the source of direction:

- Calm application UI, not a landing page.
- Existing white/slate surfaces in light mode and slate/dark surfaces in dark mode.
- Blue is the primary action color.
- Semantic colors have operational meaning only: green complete/healthy, amber waiting or capacity warning, red failed/destructive, violet split-specific state.
- Use existing rounded cards and compact controls. Do not introduce gradients, glows, decorative grids, generic illustrations, or product-marketing sections.
- Use existing typography and icon set. Icons explain actions; labels remain explicit for destructive or security-sensitive actions.

## Layout

- Prioritize current operational work before historical/configuration information.
- Transfer pages: active work first, capacity/reservations second, OAuth quota third.
- Security and destructive controls stay visually separated from routine actions.
- Long account emails, file names, limits, and timestamps must wrap or truncate safely, never force horizontal scrolling.
- Mobile uses stacked rows and full-width actions. Minimum interactive target is 44px.

## Interaction

- Every control must show loading, success, error, disabled, and empty states where relevant.
- Confirm destructive actions and OAuth migration.
- Modal/dialog flows must close with Escape and preserve visible keyboard focus.
- Use motion only for immediate feedback: progress updates, spinner, panel state. No decorative animation.

## Dials

ENERGY 1 / RHYTHM 1 / MOTION 1

Reason: PanDrive users are handling storage and transfers; visual calm, predictable layout, and minimal motion make status and failures easier to read.

## Major decision reasons

- Existing blue action color: it already identifies routine actions without competing with semantic failure states.
- Card surfaces: group operational information on dense file-management screens.
- Semantic status colors: make quota, transfer, and integrity states scannable, with text labels so color is never the only signal.
- Stacked mobile rows: account names and transfer metadata vary in length and must remain usable on narrow screens.
- Minimal motion: transfer progress is already dynamic information; extra animation would obscure it.
