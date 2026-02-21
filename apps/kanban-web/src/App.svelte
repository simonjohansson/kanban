<script lang="ts">
  import { onDestroy, onMount } from 'svelte';
  import { DefaultService, OpenAPI, type Card, type CardSummary, type Project } from './lib/generated';
  import CardDetailsModal from './lib/components/CardDetailsModal.svelte';

  type ProjectSummary = Pick<Project, 'name' | 'slug' | 'local_path' | 'remote_url'>;
  const LANES = ['Todo', 'Doing', 'Review', 'Done'] as const;
  type LaneStatus = (typeof LANES)[number];
  type ReviewReasonStatus = 'Todo' | 'Doing';
  type HistoryMode = 'push' | 'replace' | 'none';
  const CARD_EVENT_PREFIX = 'card.';
  const PROJECT_EVENT_TYPES = new Set(['project.created', 'project.deleted']);
  const CARD_ROUTE_RE = /^\/card\/([^/]+)\/(\d+)\/?$/;

  const configuredBase = (import.meta.env.VITE_KANBAN_SERVER_URL ?? '').trim();
  let resolvedBase = configuredBase;

  let projects: ProjectSummary[] = [];
  let cards: CardSummary[] = [];
  let cardsByLane: Record<LaneStatus, CardSummary[]> = emptyLaneMap();
  let selectedProjectSlug: string | null = null;
  let alertMessage: string | null = null;
  let sidebarHidden = false;
  let ws: WebSocket | null = null;
  let cardDetailsProjectSlug: string | null = null;
  let cardDetailsNumber: number | null = null;
  let cardDetails: Card | null = null;
  let cardDetailsLoading = false;
  let cardDetailsError: string | null = null;
  let cardDetailsRequestToken = 0;
  let cardDetailsOpen = false;
  let reviewActionBusy = false;
  let reviewReasonTargetStatus: ReviewReasonStatus | null = null;
  let reviewReasonInput = '';
  let reviewReasonError: string | null = null;

  $: cardsByLane = buildLaneMap(cards);
  $: cardDetailsOpen = cardDetailsProjectSlug !== null && cardDetailsNumber !== null;

  onMount(async () => {
    resolvedBase = await resolveServerBase();
    OpenAPI.BASE = resolvedBase;
    await loadProjects();
    const initialCardRoute = parseCardRoute(window.location.pathname);
    if (initialCardRoute) {
      await openCardDetails(initialCardRoute.projectSlug, initialCardRoute.number, {
        historyMode: 'replace',
        selectProject: true,
      });
    }
    window.addEventListener('popstate', handlePopState);
    window.addEventListener('keydown', handleGlobalKeyDown);
    connectWebSocket();
  });

  onDestroy(() => {
    ws?.close();
    window.removeEventListener('popstate', handlePopState);
    window.removeEventListener('keydown', handleGlobalKeyDown);
  });

  async function loadProjects(): Promise<void> {
    try {
      const payload = await DefaultService.listProjects();
      projects = [...payload.projects].sort(sortProjects);
      if (selectedProjectSlug && !projects.some((project) => project.slug === selectedProjectSlug)) {
        selectedProjectSlug = null;
      }
      if (cardDetailsProjectSlug && !projects.some((project) => project.slug === cardDetailsProjectSlug)) {
        closeCardDetails({ historyMode: 'replace' });
      }
      await loadCards();
    } catch (err) {
      alertMessage = `Failed to load projects: ${String(err)}`;
    }
  }

  async function loadCards(): Promise<void> {
    if (!selectedProjectSlug) {
      cards = [];
      return;
    }
    try {
      const payload = await DefaultService.listCards(selectedProjectSlug);
      cards = [...payload.cards].sort(sortCards);
    } catch (err) {
      alertMessage = `Failed to load cards: ${String(err)}`;
    }
  }

  async function selectProject(slug: string): Promise<void> {
    selectedProjectSlug = slug;
    if (cardDetailsProjectSlug && cardDetailsProjectSlug !== slug) {
      closeCardDetails({ historyMode: 'replace' });
    }
    await loadCards();
  }

  async function openCardDetails(
    projectSlug: string,
    number: number,
    options: { historyMode?: HistoryMode; selectProject?: boolean } = {}
  ): Promise<void> {
    const historyMode = options.historyMode ?? 'push';
    const selectProjectIfNeeded = options.selectProject ?? false;
    const requestToken = ++cardDetailsRequestToken;

    cardDetailsProjectSlug = projectSlug;
    cardDetailsNumber = number;
    cardDetails = null;
    cardDetailsError = null;
    cardDetailsLoading = true;
    syncCardDetailsRoute(projectSlug, number, historyMode);

    if (selectProjectIfNeeded && selectedProjectSlug !== projectSlug) {
      selectedProjectSlug = projectSlug;
      await loadCards();
      if (requestToken !== cardDetailsRequestToken) {
        return;
      }
    }

    try {
      const payload = await DefaultService.getCard(projectSlug, number);
      if (requestToken !== cardDetailsRequestToken) {
        return;
      }
      cardDetails = payload;
    } catch (err) {
      if (requestToken !== cardDetailsRequestToken) {
        return;
      }
      cardDetailsError = `Failed to load card details: ${String(err)}`;
    } finally {
      if (requestToken === cardDetailsRequestToken) {
        cardDetailsLoading = false;
      }
    }
  }

  async function retryCardDetails(): Promise<void> {
    if (!cardDetailsProjectSlug || cardDetailsNumber === null) {
      return;
    }
    await openCardDetails(cardDetailsProjectSlug, cardDetailsNumber, { historyMode: 'replace', selectProject: true });
  }

  function requiresReviewReason(status: LaneStatus): status is ReviewReasonStatus {
    return status === 'Todo' || status === 'Doing';
  }

  function openReviewReasonPrompt(status: ReviewReasonStatus): void {
    reviewReasonTargetStatus = status;
    reviewReasonInput = '';
    reviewReasonError = null;
  }

  function closeReviewReasonPrompt(): void {
    reviewReasonTargetStatus = null;
    reviewReasonInput = '';
    reviewReasonError = null;
  }

  function handleReviewReasonInput(value: string): void {
    reviewReasonInput = value;
    if (reviewReasonError) {
      reviewReasonError = null;
    }
  }

  async function refreshBoardAndSelectedCard(): Promise<void> {
    await loadCards();
    await retryCardDetails();
  }

  async function executeReviewMove(targetStatus: LaneStatus, reason: string | null): Promise<void> {
    if (!cardDetailsProjectSlug || cardDetailsNumber === null) {
      return;
    }
    const transitionProjectSlug = cardDetailsProjectSlug;
    const transitionCardNumber = cardDetailsNumber;
    reviewActionBusy = true;
    try {
      await DefaultService.moveCard(transitionProjectSlug, transitionCardNumber, { status: targetStatus });
      if (reason !== null) {
        const commentBody = `Moved back to ${targetStatus}: ${reason}`;
        try {
          await DefaultService.commentCard(transitionProjectSlug, transitionCardNumber, { body: commentBody });
        } catch {
          try {
            await DefaultService.moveCard(transitionProjectSlug, transitionCardNumber, { status: 'Review' });
          } catch {
            // noop; error state is shared for both failure modes
          }
          alertMessage = 'Failed to add transition reason';
          closeReviewReasonPrompt();
          return;
        }
      }
      closeReviewReasonPrompt();
      closeCardDetails({ historyMode: 'replace' });
    } catch (err) {
      alertMessage = `Failed to move card: ${String(err)}`;
    } finally {
      await refreshBoardAndSelectedCard();
      reviewActionBusy = false;
    }
  }

  async function moveReviewCard(targetStatus: LaneStatus): Promise<void> {
    if (cardDetails?.status !== 'Review') {
      return;
    }
    if (requiresReviewReason(targetStatus)) {
      openReviewReasonPrompt(targetStatus);
      return;
    }
    await executeReviewMove(targetStatus, null);
  }

  async function submitReviewReason(): Promise<void> {
    if (!reviewReasonTargetStatus) {
      return;
    }
    const trimmedReason = reviewReasonInput.trim();
    if (!trimmedReason) {
      reviewReasonError = 'Reason is required';
      return;
    }
    await executeReviewMove(reviewReasonTargetStatus, trimmedReason);
  }

  function closeCardDetails(options: { historyMode?: HistoryMode } = {}): void {
    const historyMode = options.historyMode ?? 'push';
    cardDetailsRequestToken += 1;
    closeReviewReasonPrompt();
    cardDetailsProjectSlug = null;
    cardDetailsNumber = null;
    cardDetails = null;
    cardDetailsError = null;
    cardDetailsLoading = false;
    syncRootRoute(historyMode);
  }

  function handlePopState(): void {
    const route = parseCardRoute(window.location.pathname);
    if (!route) {
      closeCardDetails({ historyMode: 'none' });
      return;
    }
    void openCardDetails(route.projectSlug, route.number, { historyMode: 'none', selectProject: true });
  }

  function handleGlobalKeyDown(event: KeyboardEvent): void {
    if (event.key === 'Escape' && reviewReasonTargetStatus) {
      event.preventDefault();
      closeReviewReasonPrompt();
      return;
    }
    if (event.key === 'Escape' && cardDetailsOpen) {
      event.preventDefault();
      closeCardDetails({ historyMode: 'push' });
    }
  }

  function connectWebSocket(): void {
    const wsURL = buildWebSocketURL();
    ws = new WebSocket(wsURL);

    ws.addEventListener('message', async (event) => {
      try {
        const payload = JSON.parse(String(event.data)) as { type?: string; project?: string };
        if (payload.type && PROJECT_EVENT_TYPES.has(payload.type)) {
          await loadProjects();
          return;
        }
        if (
          payload.type &&
          payload.type.startsWith(CARD_EVENT_PREFIX) &&
          selectedProjectSlug &&
          payload.project === selectedProjectSlug
        ) {
          await loadCards();
          if (cardDetailsProjectSlug === selectedProjectSlug && cardDetailsNumber !== null) {
            await retryCardDetails();
          }
        }
      } catch {
        // ignore malformed event payloads
      }
    });

    ws.addEventListener('error', () => {
      alertMessage = 'Project stream failed: websocket error';
    });
  }

  function buildWebSocketURL(): string {
    const base = resolvedBase || window.location.origin;
    const parsed = new URL(base);
    parsed.protocol = parsed.protocol === 'https:' ? 'wss:' : 'ws:';
    parsed.pathname = '/ws';
    parsed.search = '';
    parsed.hash = '';
    return parsed.toString();
  }

  function sortProjects(lhs: ProjectSummary, rhs: ProjectSummary): number {
    const byName = lhs.name.localeCompare(rhs.name);
    if (byName !== 0) return byName;
    return lhs.slug.localeCompare(rhs.slug);
  }

  function sortCards(lhs: CardSummary, rhs: CardSummary): number {
    return lhs.number - rhs.number;
  }

  function emptyLaneMap(): Record<LaneStatus, CardSummary[]> {
    return {
      Todo: [],
      Doing: [],
      Review: [],
      Done: [],
    };
  }

  function buildLaneMap(source: CardSummary[]): Record<LaneStatus, CardSummary[]> {
    const map = emptyLaneMap();
    for (const card of source) {
      if (card.status === 'Todo' || card.status === 'Doing' || card.status === 'Review' || card.status === 'Done') {
        map[card.status].push(card);
      }
    }
    return map;
  }

  function tooltip(project: ProjectSummary): string {
    const lines: string[] = [];
    if (project.local_path?.trim()) lines.push(`Local: ${project.local_path.trim()}`);
    if (project.remote_url?.trim()) lines.push(`Remote: ${project.remote_url.trim()}`);
    return lines.length > 0 ? lines.join('\n') : project.slug;
  }

  function checklistSummary(card: CardSummary): string | null {
    const segments: string[] = [];
    if (card.todos_count > 0) {
      segments.push(`${card.todos_completed_count}/${card.todos_count} Todos`);
    }
    if (card.acceptance_criteria_count > 0) {
      segments.push(`${card.acceptance_criteria_completed_count}/${card.acceptance_criteria_count} AC`);
    }
    if (segments.length === 0) {
      return null;
    }
    return segments.join(' ');
  }

  function toggleSidebar(): void {
    sidebarHidden = !sidebarHidden;
  }

  function parseCardRoute(pathname: string): { projectSlug: string; number: number } | null {
    const match = pathname.match(CARD_ROUTE_RE);
    if (!match) {
      return null;
    }
    let projectSlug: string;
    try {
      projectSlug = decodeURIComponent(match[1]);
    } catch {
      return null;
    }
    const number = Number.parseInt(match[2], 10);
    if (!Number.isInteger(number) || number <= 0) {
      return null;
    }
    return { projectSlug, number };
  }

  function syncCardDetailsRoute(projectSlug: string, number: number, mode: HistoryMode): void {
    if (mode === 'none') {
      return;
    }
    const nextPath = `/card/${encodeURIComponent(projectSlug)}/${number}`;
    if (window.location.pathname === nextPath) {
      return;
    }
    if (mode === 'replace') {
      window.history.replaceState({}, '', nextPath);
      return;
    }
    window.history.pushState({}, '', nextPath);
  }

  function syncRootRoute(mode: HistoryMode): void {
    if (mode === 'none') {
      return;
    }
    if (window.location.pathname === '/') {
      return;
    }
    if (mode === 'replace') {
      window.history.replaceState({}, '', '/');
      return;
    }
    window.history.pushState({}, '', '/');
  }

  async function resolveServerBase(): Promise<string> {
    if (configuredBase) {
      return configuredBase;
    }
    if (import.meta.env.DEV) {
      return window.location.origin;
    }

    try {
      const response = await fetch('/client-config');
      if (response.ok) {
        const payload = (await response.json()) as { server_url?: unknown };
        if (typeof payload.server_url === 'string' && payload.server_url.trim() !== '') {
          return payload.server_url.trim();
        }
      }
    } catch {
      // fallback to same-origin
    }

    return window.location.origin;
  }
</script>

<main class="layout" class:sidebar-hidden={sidebarHidden}>
  <aside class="sidebar" class:collapsed={sidebarHidden} data-testid="projects-sidebar">
    <div class="sidebar-header">
      {#if !sidebarHidden}
        <h1 data-testid="projects-title">Projects</h1>
      {/if}
      <button
        aria-label={sidebarHidden ? 'Show projects' : 'Hide projects'}
        class="sidebar-toggle"
        data-testid="sidebar-toggle"
        on:click={toggleSidebar}
        type="button"
      >
        {sidebarHidden ? '›' : '‹'}
      </button>
    </div>
    {#if !sidebarHidden}
      <ul class="project-list">
        {#each projects as project (project.slug)}
          <li>
            <button
              class:selected={selectedProjectSlug === project.slug}
              data-testid="project-item"
              on:click={() => selectProject(project.slug)}
              title={tooltip(project)}
              type="button"
            >
              {project.name}
            </button>
          </li>
        {/each}
      </ul>
    {/if}
  </aside>
  <section class="detail">
    {#if !selectedProjectSlug}
      <p>No project selected</p>
    {:else}
      <div class="board" data-testid="board">
        {#each LANES as lane}
          <section class="lane" data-testid={'lane-' + lane}>
            <h2>{lane}</h2>
            <div class="lane-cards">
              {#each cardsByLane[lane] as card (card.id)}
                <article
                  class="card"
                  data-testid="card-item"
                  on:click={() => openCardDetails(card.project, card.number, { historyMode: 'push', selectProject: false })}
                >
                  <div class="card-title">{card.title}</div>
                  {#if checklistSummary(card)}
                    <div class="card-checklists">{checklistSummary(card)}</div>
                  {/if}
                  {#if card.branch?.trim()}
                    <div class="card-branch">{card.branch.trim()}</div>
                  {/if}
                </article>
              {/each}
            </div>
          </section>
        {/each}
      </div>
    {/if}
  </section>
</main>

{#if cardDetailsOpen}
  <CardDetailsModal
    card={cardDetails}
    errorMessage={cardDetailsError}
    loading={cardDetailsLoading}
    onCancelReviewReason={closeReviewReasonPrompt}
    onClose={() => closeCardDetails({ historyMode: 'push' })}
    onMoveReviewToDoing={() => moveReviewCard('Doing')}
    onMoveReviewToDone={() => moveReviewCard('Done')}
    onMoveReviewToTodo={() => moveReviewCard('Todo')}
    onReviewReasonInput={handleReviewReasonInput}
    onRetry={retryCardDetails}
    onSubmitReviewReason={submitReviewReason}
    reviewActionBusy={reviewActionBusy}
    reviewActionsVisible={cardDetails?.status === 'Review'}
    reviewReasonError={reviewReasonError}
    reviewReasonInput={reviewReasonInput}
    reviewReasonTargetStatus={reviewReasonTargetStatus}
  />
{/if}

{#if alertMessage}
  <div class="alert" role="alert">{alertMessage}</div>
{/if}

<style>
  :global(body) {
    margin: 0;
    font-family: ui-sans-serif, system-ui, -apple-system, Segoe UI, Roboto, Helvetica, Arial, sans-serif;
    color: #1f2937;
  }

  .layout {
    --sidebar-expanded-width: 240px;
    --sidebar-collapsed-width: 64px;
    display: grid;
    grid-template-columns: var(--sidebar-expanded-width) minmax(0, 1fr);
    min-height: 100vh;
    background: linear-gradient(180deg, #faf7f0 0%, #ffffff 100%);
  }

  .layout.sidebar-hidden {
    grid-template-columns: var(--sidebar-collapsed-width) minmax(0, 1fr);
  }

  .sidebar {
    position: relative;
    border-right: 1px solid #e5e7eb;
    padding: 16px;
    background: #fff;
    overflow: hidden;
  }

  .sidebar.collapsed {
    padding-inline: 10px;
  }

  .sidebar-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
    margin: 0 0 16px 0;
  }

  .sidebar h1 {
    margin: 0;
    font-size: 1.05rem;
    letter-spacing: 0.02em;
  }

  .sidebar.collapsed h1 {
    display: none;
  }

  .sidebar-toggle {
    border: 1px solid #0f766e;
    background: #0f766e;
    color: #ffffff;
    border-radius: 8px;
    width: 34px;
    height: 34px;
    cursor: pointer;
    font-size: 1.1rem;
    font-weight: 700;
    line-height: 1;
    box-shadow: 0 1px 3px rgba(15, 118, 110, 0.35);
  }

  .sidebar-toggle:hover {
    background: #115e59;
    border-color: #115e59;
  }

  .project-list {
    list-style: none;
    padding: 0;
    margin: 0;
    display: grid;
    gap: 8px;
  }

  .project-list button {
    width: 100%;
    text-align: left;
    border: 1px solid #e5e7eb;
    border-radius: 10px;
    padding: 10px 12px;
    background: #f9fafb;
    cursor: pointer;
  }

  .project-list button.selected {
    border-color: #0f766e;
    background: #ecfeff;
    color: #134e4a;
    font-weight: 600;
  }

  .detail {
    padding: 16px;
    color: #6b7280;
    overflow: hidden;
  }

  .board {
    display: flex;
    gap: 12px;
    align-items: stretch;
    justify-content: stretch;
    width: 100%;
    min-width: 0;
    min-height: calc(100vh - 48px);
  }

  .board > * {
    flex: 1 1 0;
  }

  .lane {
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 12px;
    background: #f3f4f6;
    border: 1px solid #e5e7eb;
    border-radius: 12px;
    padding: 10px;
  }

  .lane h2 {
    margin: 0 0 10px 0;
    font-size: 0.95rem;
  }

  .lane-cards {
    display: grid;
    gap: 8px;
  }

  .card {
    background: #fff;
    border: 1px solid #d1d5db;
    border-radius: 10px;
    padding: 10px;
    color: #111827;
    font-size: 0.9rem;
    overflow-wrap: anywhere;
    display: grid;
    gap: 6px;
    cursor: pointer;
  }

  .card-title {
    line-height: 1.2;
  }

  .card-branch {
    font-size: 0.78rem;
    line-height: 1.2;
    font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, Liberation Mono, Courier New, monospace;
    color: #4b5563;
  }

  .card-checklists {
    font-size: 0.78rem;
    line-height: 1.2;
    font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, Liberation Mono, Courier New, monospace;
    color: #4b5563;
  }

  .alert {
    position: fixed;
    right: 16px;
    bottom: 16px;
    background: #991b1b;
    color: #fff;
    padding: 10px 12px;
    border-radius: 8px;
    max-width: 540px;
    font-size: 0.9rem;
  }

</style>
