<script lang="ts">
  import { onDestroy, onMount } from 'svelte';
  import { DefaultService, OpenAPI, type CardSummary, type Project } from './lib/generated';

  type ProjectSummary = Pick<Project, 'name' | 'slug' | 'local_path' | 'remote_url'>;
  const LANES = ['Todo', 'Doing', 'Review', 'Done'] as const;
  type LaneStatus = (typeof LANES)[number];
  const CARD_EVENT_TYPES = new Set(['card.created', 'card.moved', 'card.deleted_soft', 'card.deleted_hard']);
  const PROJECT_EVENT_TYPES = new Set(['project.created', 'project.deleted']);

  const configuredBase = (import.meta.env.VITE_KANBAN_SERVER_URL ?? '').trim();
  OpenAPI.BASE = configuredBase;

  let projects: ProjectSummary[] = [];
  let cards: CardSummary[] = [];
  let cardsByLane: Record<LaneStatus, CardSummary[]> = emptyLaneMap();
  let selectedProjectSlug: string | null = null;
  let alertMessage: string | null = null;
  let sidebarHidden = false;
  let ws: WebSocket | null = null;

  $: cardsByLane = buildLaneMap(cards);

  onMount(async () => {
    await loadProjects();
    connectWebSocket();
  });

  onDestroy(() => {
    ws?.close();
  });

  async function loadProjects(): Promise<void> {
    try {
      const payload = await DefaultService.listProjects();
      projects = [...payload.projects].sort(sortProjects);
      if (selectedProjectSlug && !projects.some((project) => project.slug === selectedProjectSlug)) {
        selectedProjectSlug = null;
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
    await loadCards();
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
          CARD_EVENT_TYPES.has(payload.type) &&
          selectedProjectSlug &&
          payload.project === selectedProjectSlug
        ) {
          await loadCards();
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
    const base = configuredBase || window.location.origin;
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

  function toggleSidebar(): void {
    sidebarHidden = !sidebarHidden;
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
                <article class="card" data-testid="card-item">{card.title}</article>
              {/each}
            </div>
          </section>
        {/each}
      </div>
    {/if}
  </section>
</main>

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
    margin: 0 0 16px 0;
    font-size: 1.05rem;
    letter-spacing: 0.02em;
  }

  .sidebar.collapsed h1 {
    display: none;
  }

  .sidebar-toggle {
    border: 1px solid #d1d5db;
    background: #f9fafb;
    color: #1f2937;
    border-radius: 8px;
    width: 32px;
    height: 32px;
    cursor: pointer;
    font-size: 1rem;
    line-height: 1;
  }

  .sidebar-toggle:hover {
    background: #eef2ff;
    border-color: #c7d2fe;
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
    display: grid;
    grid-template-columns: repeat(4, minmax(0, 1fr));
    gap: 12px;
    align-items: start;
    width: 100%;
    min-height: calc(100vh - 48px);
  }

  .lane {
    min-width: 0;
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
