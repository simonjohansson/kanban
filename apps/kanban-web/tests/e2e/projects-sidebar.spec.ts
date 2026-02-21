import { expect, test } from '@playwright/test';
import { spawn, type ChildProcessWithoutNullStreams } from 'node:child_process';
import fs from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const currentDir = path.dirname(fileURLToPath(import.meta.url));
const backendRoot = path.resolve(currentDir, '../../../../backend');

let backendProc: ChildProcessWithoutNullStreams | undefined;

async function waitForHealth(url: string, timeoutMs: number): Promise<void> {
  const start = Date.now();
  while (Date.now() - start < timeoutMs) {
    try {
      const response = await fetch(url);
      if (response.status === 200) return;
    } catch {
      // retry
    }
    await new Promise((resolve) => setTimeout(resolve, 200));
  }
  throw new Error(`backend not healthy within ${timeoutMs}ms`);
}

test.beforeAll(async () => {
  const root = await fs.mkdtemp(path.join(os.tmpdir(), 'kanban-web-e2e-'));
  const cards = path.join(root, 'cards');
  const sqlite = path.join(root, 'projection.db');
  await fs.mkdir(cards, { recursive: true });

  backendProc = spawn(
    '/usr/bin/env',
    ['go', 'run', './cmd/kanban', 'serve', '--addr', '127.0.0.1:18080', '--cards-path', cards, '--sqlite-path', sqlite],
    {
      cwd: backendRoot,
      stdio: 'pipe',
      env: { ...process.env, PATH: process.env.PATH ?? '' },
    }
  );

  backendProc.stdout.on('data', (chunk) => {
    process.stdout.write(`[backend] ${chunk}`);
  });
  backendProc.stderr.on('data', (chunk) => {
    process.stdout.write(`[backend] ${chunk}`);
  });

  await waitForHealth('http://127.0.0.1:18080/health', 20000);
});

test.afterAll(async () => {
  if (!backendProc) return;
  backendProc.kill('SIGTERM');
  await new Promise<void>((resolve) => {
    backendProc?.on('exit', () => resolve());
    setTimeout(resolve, 4000);
  });
});

test('shows 4 lanes and reflects cards across project switching and moves', async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 820 });
  await page.goto('/');

  await expect(page.getByTestId('projects-title')).toHaveText('Projects');
  await expect(page.getByTestId('project-item')).toHaveCount(0);

  const createAlphaResponse = await fetch('http://127.0.0.1:18080/projects', {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify({ name: 'Alpha Project' }),
  });
  expect(createAlphaResponse.status).toBe(201);

  const createBetaResponse = await fetch('http://127.0.0.1:18080/projects', {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify({ name: 'Beta Project' }),
  });
  expect(createBetaResponse.status).toBe(201);

  await expect(page.getByTestId('project-item')).toHaveCount(2);

  const createAlphaCardResponse = await fetch('http://127.0.0.1:18080/projects/alpha-project/cards', {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify({ title: 'Alpha Task', description: 'alpha', branch: 'feature/alpha-task', status: 'Todo' }),
  });
  expect(createAlphaCardResponse.status).toBe(201);

  const createBetaCardResponse = await fetch('http://127.0.0.1:18080/projects/beta-project/cards', {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify({ title: 'Beta Task', description: 'beta', status: 'Todo' }),
  });
  expect(createBetaCardResponse.status).toBe(201);

  await page.getByTestId('project-item').filter({ hasText: 'Alpha Project' }).click();
  await expect(page.getByTestId('lane-Todo')).toContainText('Alpha Task');
  await expect(page.getByTestId('lane-Todo')).toContainText('feature/alpha-task');
  await expect(page.getByTestId('lane-Doing')).not.toContainText('Alpha Task');
  await expect(page.getByTestId('lane-Review')).not.toContainText('Alpha Task');
  await expect(page.getByTestId('lane-Done')).not.toContainText('Alpha Task');

  await page.getByTestId('project-item').filter({ hasText: 'Beta Project' }).click();
  await expect(page.getByTestId('lane-Todo')).toContainText('Beta Task');
  await expect(page.getByTestId('lane-Todo')).not.toContainText('Alpha Task');

  await page.getByTestId('project-item').filter({ hasText: 'Alpha Project' }).click();
  await expect(page.getByTestId('lane-Todo')).toContainText('Alpha Task');
  await expect(page.getByTestId('lane-Todo')).not.toContainText('Beta Task');

  const moveAlphaResponse = await fetch('http://127.0.0.1:18080/projects/alpha-project/cards/1/move', {
    method: 'PATCH',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify({ status: 'Done' }),
  });
  expect(moveAlphaResponse.status).toBe(200);

  await expect(page.getByTestId('lane-Todo')).not.toContainText('Alpha Task');
  await expect(page.getByTestId('lane-Done')).toContainText('Alpha Task');

  const sidebar = page.getByTestId('projects-sidebar');
  const toggle = page.getByTestId('sidebar-toggle');

  await expect(toggle).toBeVisible();

  const sidebarExpanded = await sidebar.boundingBox();
  expect(sidebarExpanded).not.toBeNull();
  expect((sidebarExpanded?.width ?? 0) >= 220).toBe(true);

  await toggle.click();

  const sidebarCollapsed = await sidebar.boundingBox();
  expect(sidebarCollapsed).not.toBeNull();
  expect((sidebarCollapsed?.width ?? 0) <= 90).toBe(true);

  await toggle.click();

  await page.setViewportSize({ width: 920, height: 820 });
  await expect(page.getByTestId('board')).toBeVisible();

  const boardUsesFlexLayout = await page.evaluate(() => {
    const board = document.querySelector('[data-testid="board"]');
    if (!board) return false;
    return window.getComputedStyle(board).display === 'flex';
  });
  expect(boardUsesFlexLayout).toBe(true);

  const boardFitsAtMediumViewport = await page.evaluate(() => {
    const board = document.querySelector('[data-testid="board"]') as HTMLElement | null;
    if (!board) return false;
    return board.scrollWidth <= board.clientWidth;
  });
  expect(boardFitsAtMediumViewport).toBe(true);

  await page.setViewportSize({ width: 760, height: 820 });

  const boardFitsAtNarrowViewport = await page.evaluate(() => {
    const board = document.querySelector('[data-testid="board"]') as HTMLElement | null;
    if (!board) return false;
    return board.scrollWidth <= board.clientWidth;
  });
  expect(boardFitsAtNarrowViewport).toBe(true);
});

test('opens card details popup with deep links and close behaviors', async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 820 });
  await page.goto('/');

  const createProjectResponse = await fetch('http://127.0.0.1:18080/projects', {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify({ name: 'Modal Project' }),
  });
  expect(createProjectResponse.status).toBe(201);

  const createFirstCardResponse = await fetch('http://127.0.0.1:18080/projects/modal-project/cards', {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify({
      title: 'Card One',
      description: 'First description body',
      branch: 'feature/card-one',
      status: 'Todo',
    }),
  });
  expect(createFirstCardResponse.status).toBe(201);

  const createSecondCardResponse = await fetch('http://127.0.0.1:18080/projects/modal-project/cards', {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify({
      title: 'Card Two',
      description: 'Second description body',
      branch: 'feature/card-two',
      status: 'Todo',
    }),
  });
  expect(createSecondCardResponse.status).toBe(201);

  const firstCommentResponse = await fetch('http://127.0.0.1:18080/projects/modal-project/cards/1/comments', {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify({ body: 'First comment line 1\\nFirst comment line 2' }),
  });
  expect(firstCommentResponse.status).toBe(200);

  const secondCommentResponse = await fetch('http://127.0.0.1:18080/projects/modal-project/cards/2/comments', {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify({ body: 'Second comment body' }),
  });
  expect(secondCommentResponse.status).toBe(200);

  const firstTodoResponse = await fetch('http://127.0.0.1:18080/projects/modal-project/cards/1/todos', {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify({ text: 'Write tests' }),
  });
  expect(firstTodoResponse.status).toBe(201);

  const secondTodoResponse = await fetch('http://127.0.0.1:18080/projects/modal-project/cards/1/todos', {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify({ text: 'Run tests' }),
  });
  expect(secondTodoResponse.status).toBe(201);

  const doneTodoResponse = await fetch('http://127.0.0.1:18080/projects/modal-project/cards/1/todos/2', {
    method: 'PATCH',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify({ completed: true }),
  });
  expect(doneTodoResponse.status).toBe(200);

  const firstAcceptanceResponse = await fetch('http://127.0.0.1:18080/projects/modal-project/cards/1/acceptance', {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify({ text: 'Requirement A' }),
  });
  expect(firstAcceptanceResponse.status).toBe(201);

  const secondAcceptanceResponse = await fetch('http://127.0.0.1:18080/projects/modal-project/cards/1/acceptance', {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify({ text: 'Requirement B' }),
  });
  expect(secondAcceptanceResponse.status).toBe(201);

  const doneAcceptanceResponse = await fetch('http://127.0.0.1:18080/projects/modal-project/cards/1/acceptance/2', {
    method: 'PATCH',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify({ completed: true }),
  });
  expect(doneAcceptanceResponse.status).toBe(200);

  await page.reload();

  await page.getByTestId('project-item').filter({ hasText: 'Modal Project' }).click();
  await expect(page.getByTestId('lane-Todo')).toContainText('Card One');
  await expect(page.getByTestId('card-item').filter({ hasText: 'Card One' })).toContainText('1/2 Todos 1/2 AC');
  await expect(page.getByTestId('lane-Todo')).toContainText('Card Two');
  await expect(page.getByTestId('card-item').filter({ hasText: 'Card Two' })).not.toContainText('Todos');
  await expect(page.getByTestId('card-item').filter({ hasText: 'Card Two' })).not.toContainText('AC');

  await page.getByTestId('card-item').filter({ hasText: 'Card One' }).click();
  await expect(page.getByTestId('card-details-modal')).toBeVisible();
  await expect(page.getByTestId('card-details-title')).toHaveText('Card One');
  await expect(page.getByTestId('card-details-branch')).toContainText('feature/card-one');
  await expect(page.getByTestId('card-details-description')).toContainText('First description body');
  await expect(page.getByTestId('card-details-comments')).toContainText('First comment line 1');
  await expect(page.getByTestId('card-details-comments')).toContainText('First comment line 2');
  await expect(page.getByTestId('card-details-comments')).not.toContainText('\\n');
  await expect(page.getByTestId('card-details-todos')).toContainText('Write tests');
  await expect(page.getByTestId('card-details-todos')).toContainText('Run tests');
  await expect(page.getByTestId('card-details-todos')).toContainText('[x]');
  await expect(page.getByTestId('card-details-acceptance-criteria')).toContainText('Requirement A');
  await expect(page.getByTestId('card-details-acceptance-criteria')).toContainText('Requirement B');
  await expect(page.getByTestId('card-details-acceptance-criteria')).toContainText('[x]');
  await expect(page).toHaveURL(/\/card\/modal-project\/1$/);

  await page.getByTestId('card-details-close').click();
  await expect(page.getByTestId('card-details-modal')).toHaveCount(0);
  await expect(page).toHaveURL('/');

  await page.getByTestId('card-item').filter({ hasText: 'Card Two' }).click();
  await expect(page.getByTestId('card-details-title')).toHaveText('Card Two');
  await expect(page.getByTestId('card-details-branch')).toContainText('feature/card-two');
  await expect(page.getByTestId('card-details-description')).toContainText('Second description body');
  await expect(page.getByTestId('card-details-comments')).toContainText('Second comment body');
  await expect(page.getByTestId('card-details-todos')).toContainText('No todos');
  await expect(page.getByTestId('card-details-acceptance-criteria')).toContainText('No acceptance criteria');
  await expect(page).toHaveURL(/\/card\/modal-project\/2$/);

  await page.getByTestId('card-details-close').click();
  await expect(page.getByTestId('card-details-modal')).toHaveCount(0);
  await expect(page).toHaveURL('/');

  await page.getByTestId('card-item').filter({ hasText: 'Card One' }).click();
  await expect(page.getByTestId('card-details-modal')).toBeVisible();
  await page.keyboard.press('Escape');
  await expect(page.getByTestId('card-details-modal')).toHaveCount(0);
  await expect(page).toHaveURL('/');

  await page.getByTestId('card-item').filter({ hasText: 'Card One' }).click();
  await expect(page.getByTestId('card-details-modal')).toBeVisible();
  await page.getByTestId('lane-Doing').click({ position: { x: 10, y: 10 } });
  await expect(page.getByTestId('card-details-modal')).toHaveCount(0);
  await expect(page).toHaveURL('/');

  await page.goto('/card/modal-project/1');
  await expect(page.getByTestId('card-details-modal')).toBeVisible();
  await expect(page.getByTestId('card-details-title')).toHaveText('Card One');

  await page.goto('/card/modal-project/999');
  await expect(page.getByTestId('card-details-modal')).toBeVisible();
  await expect(page.getByTestId('card-details-error')).toContainText('Failed to load card details');
  await page.getByTestId('card-details-retry').click();
  await expect(page.getByTestId('card-details-error')).toContainText('Failed to load card details');
});

test('guards malformed deep links and ignores stale card-detail requests', async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 820 });
  await page.goto('/');

  const createRaceAResponse = await fetch('http://127.0.0.1:18080/projects', {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify({ name: 'Race A' }),
  });
  expect(createRaceAResponse.status).toBe(201);

  const createRaceBResponse = await fetch('http://127.0.0.1:18080/projects', {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify({ name: 'Race B' }),
  });
  expect(createRaceBResponse.status).toBe(201);

  const createRaceACardResponse = await fetch('http://127.0.0.1:18080/projects/race-a/cards', {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify({ title: 'Race A Card', description: 'a', status: 'Todo' }),
  });
  expect(createRaceACardResponse.status).toBe(201);

  const createRaceBCardResponse = await fetch('http://127.0.0.1:18080/projects/race-b/cards', {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify({ title: 'Race B Card', description: 'b', status: 'Todo' }),
  });
  expect(createRaceBCardResponse.status).toBe(201);

  await page.reload();
  await expect(page.getByTestId('project-item').filter({ hasText: 'Race A' })).toHaveCount(1);
  await expect(page.getByTestId('project-item').filter({ hasText: 'Race B' })).toHaveCount(1);

  await page.route('**/projects/race-a/cards', async (route) => {
    await new Promise((resolve) => setTimeout(resolve, 500));
    await route.continue();
  });

  await page.evaluate(() => {
    window.history.pushState({}, '', '/card/race-a/1');
    window.dispatchEvent(new PopStateEvent('popstate'));
    window.history.pushState({}, '', '/card/race-b/1');
    window.dispatchEvent(new PopStateEvent('popstate'));
  });

  await expect(page.getByTestId('card-details-modal')).toBeVisible();
  await expect(page.getByTestId('card-details-title')).toHaveText('Race B Card');
  await expect(page).toHaveURL(/\/card\/race-b\/1$/);

  await page.waitForTimeout(700);
  await expect(page.getByTestId('card-details-title')).toHaveText('Race B Card');

  const pageErrors: Error[] = [];
  page.on('pageerror', (error) => pageErrors.push(error));

  await page.evaluate(() => {
    window.history.pushState({}, '', '/card/%/1');
    window.dispatchEvent(new PopStateEvent('popstate'));
  });
  await page.waitForTimeout(120);

  expect(pageErrors).toHaveLength(0);
  await page.getByTestId('project-item').filter({ hasText: 'Race A' }).click();
  await expect(page.getByTestId('lane-Todo')).toContainText('Race A Card');
});

test('live-updates open card details when todos and acceptance criteria change', async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 820 });
  await page.goto('/');

  const createProjectResponse = await fetch('http://127.0.0.1:18080/projects', {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify({ name: 'Live Update Project' }),
  });
  expect(createProjectResponse.status).toBe(201);

  const createCardResponse = await fetch('http://127.0.0.1:18080/projects/live-update-project/cards', {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify({
      title: 'Observe live updates',
      description: 'watch todo/ac changes without closing modal',
      status: 'Todo',
    }),
  });
  expect(createCardResponse.status).toBe(201);

  await page.reload();
  await page.getByTestId('project-item').filter({ hasText: 'Live Update Project' }).click();
  await page.getByTestId('card-item').filter({ hasText: 'Observe live updates' }).click();
  await expect(page.getByTestId('card-details-modal')).toBeVisible();
  await expect(page.getByTestId('card-details-todos')).toContainText('No todos');
  await expect(page.getByTestId('card-details-acceptance-criteria')).toContainText('No acceptance criteria');

  const addTodoResponse = await fetch('http://127.0.0.1:18080/projects/live-update-project/cards/1/todos', {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify({ text: 'Live todo item' }),
  });
  expect(addTodoResponse.status).toBe(201);
  const addedTodo = (await addTodoResponse.json()) as { id: number };

  await expect(page.getByTestId('card-details-todos')).toContainText('Live todo item');

  const addAcceptanceResponse = await fetch('http://127.0.0.1:18080/projects/live-update-project/cards/1/acceptance', {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify({ text: 'Live AC item' }),
  });
  expect(addAcceptanceResponse.status).toBe(201);
  const addedAcceptance = (await addAcceptanceResponse.json()) as { id: number };

  await expect(page.getByTestId('card-details-acceptance-criteria')).toContainText('Live AC item');

  const doneTodoResponse = await fetch(
    `http://127.0.0.1:18080/projects/live-update-project/cards/1/todos/${addedTodo.id}`,
    {
      method: 'PATCH',
      headers: { 'content-type': 'application/json' },
      body: JSON.stringify({ completed: true }),
    }
  );
  expect(doneTodoResponse.status).toBe(200);

  const doneAcceptanceResponse = await fetch(
    `http://127.0.0.1:18080/projects/live-update-project/cards/1/acceptance/${addedAcceptance.id}`,
    {
      method: 'PATCH',
      headers: { 'content-type': 'application/json' },
      body: JSON.stringify({ completed: true }),
    }
  );
  expect(doneAcceptanceResponse.status).toBe(200);

  await expect(page.getByTestId('card-item').filter({ hasText: 'Observe live updates' })).toContainText('1/1 Todos 1/1 AC');
});

test('requires reason for Review to Todo/Doing and rolls back when comment fails', async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 820 });
  await page.goto('/');

  const createProjectResponse = await fetch('http://127.0.0.1:18080/projects', {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify({ name: 'Review Flow Project' }),
  });
  expect(createProjectResponse.status).toBe(201);

  const createCardResponse = await fetch('http://127.0.0.1:18080/projects/review-flow-project/cards', {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify({
      title: 'Review Flow Card',
      description: 'Review card body',
      status: 'Review',
    }),
  });
  expect(createCardResponse.status).toBe(201);

  await page.reload();
  await page.getByTestId('project-item').filter({ hasText: 'Review Flow Project' }).click();
  await expect(page.getByTestId('lane-Review')).toContainText('Review Flow Card');

  await page.getByTestId('card-item').filter({ hasText: 'Review Flow Card' }).click();
  await expect(page.getByTestId('card-details-modal')).toBeVisible();
  await expect(page.getByTestId('card-review-actions')).toBeVisible();
  await expect(page.getByTestId('card-review-move-todo')).toBeVisible();
  await expect(page.getByTestId('card-review-move-doing')).toBeVisible();
  await expect(page.getByTestId('card-review-move-done')).toBeVisible();

  await page.getByTestId('card-review-move-doing').click();
  await expect(page.getByTestId('review-reason-modal')).toBeVisible();
  await expect(page.getByTestId('review-reason-input')).toBeFocused();
  await page.getByTestId('review-reason-submit').click();
  await expect(page.getByTestId('review-reason-error')).toContainText('Reason is required');
  await page.getByTestId('review-reason-input').fill('Address QA feedback');
  await page.getByTestId('review-reason-submit').click();

  await expect(page.getByTestId('lane-Review')).not.toContainText('Review Flow Card');
  await expect(page.getByTestId('lane-Doing')).toContainText('Review Flow Card');
  await expect(page.getByTestId('card-details-modal')).toHaveCount(0);
  const movedToDoing = await fetch('http://127.0.0.1:18080/projects/review-flow-project/cards/1');
  expect(movedToDoing.status).toBe(200);
  const movedToDoingBody = (await movedToDoing.json()) as { comments?: Array<{ body?: string }> };
  const doingComment = movedToDoingBody.comments?.[movedToDoingBody.comments.length - 1]?.body;
  expect(doingComment).toBe('Moved back to Doing: Address QA feedback');

  const moveBackToReview = await fetch('http://127.0.0.1:18080/projects/review-flow-project/cards/1/move', {
    method: 'PATCH',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify({ status: 'Review' }),
  });
  expect(moveBackToReview.status).toBe(200);
  await expect(page.getByTestId('lane-Review')).toContainText('Review Flow Card');
  await page.getByTestId('card-item').filter({ hasText: 'Review Flow Card' }).click();
  await expect(page.getByTestId('card-details-modal')).toBeVisible();

  await page.getByTestId('card-review-move-todo').click();
  await expect(page.getByTestId('review-reason-modal')).toBeVisible();
  await expect(page.getByTestId('review-reason-input')).toBeFocused();
  await page.getByTestId('review-reason-input').fill('Should be canceled');
  await page.getByTestId('review-reason-cancel').click();
  await expect(page.getByTestId('review-reason-modal')).toHaveCount(0);
  await expect(page.getByTestId('lane-Review')).toContainText('Review Flow Card');

  const afterCancel = await fetch('http://127.0.0.1:18080/projects/review-flow-project/cards/1');
  expect(afterCancel.status).toBe(200);
  const afterCancelBody = (await afterCancel.json()) as { comments?: Array<{ body?: string }> };
  expect(afterCancelBody.comments?.length).toBe(1);

  await page.route('**/projects/review-flow-project/cards/1/comments', async (route) => {
    await route.fulfill({
      status: 500,
      contentType: 'application/json',
      body: JSON.stringify({ status: 500, error: 'forced comment failure for test' }),
    });
  });

  await page.getByTestId('card-review-move-doing').click();
  await expect(page.getByTestId('review-reason-modal')).toBeVisible();
  await page.getByTestId('review-reason-input').fill('Will fail');
  await page.getByTestId('review-reason-submit').click();
  await expect(page.getByRole('alert')).toContainText('Failed to add transition reason');
  await expect(page.getByTestId('lane-Review')).toContainText('Review Flow Card');
  await expect(page.getByTestId('lane-Doing')).not.toContainText('Review Flow Card');
  await page.unroute('**/projects/review-flow-project/cards/1/comments');

  await page.getByTestId('card-review-move-done').click();
  await expect(page.getByTestId('card-details-modal')).toHaveCount(0);
  await expect(page.getByTestId('review-reason-modal')).toHaveCount(0);
  await expect(page.getByTestId('lane-Done')).toContainText('Review Flow Card');
  await expect(page.getByTestId('lane-Review')).not.toContainText('Review Flow Card');

  const movedToDone = await fetch('http://127.0.0.1:18080/projects/review-flow-project/cards/1');
  expect(movedToDone.status).toBe(200);
  const movedToDoneBody = (await movedToDone.json()) as { comments?: Array<{ body?: string }> };
  expect(movedToDoneBody.comments?.length).toBe(1);
});

test('review reason comment stays on original card when selection changes mid-transition', async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 820 });
  await page.goto('/');

  const createProjectResponse = await fetch('http://127.0.0.1:18080/projects', {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify({ name: 'Review Race Project' }),
  });
  expect(createProjectResponse.status).toBe(201);

  const createCardOneResponse = await fetch('http://127.0.0.1:18080/projects/review-race-project/cards', {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify({ title: 'Race Card One', description: 'one', status: 'Review' }),
  });
  expect(createCardOneResponse.status).toBe(201);

  const createCardTwoResponse = await fetch('http://127.0.0.1:18080/projects/review-race-project/cards', {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify({ title: 'Race Card Two', description: 'two', status: 'Review' }),
  });
  expect(createCardTwoResponse.status).toBe(201);

  await page.reload();
  await page.getByTestId('project-item').filter({ hasText: 'Review Race Project' }).click();
  await expect(page.getByTestId('lane-Review')).toContainText('Race Card One');
  await expect(page.getByTestId('lane-Review')).toContainText('Race Card Two');

  let commentRequestURL = '';
  await page.route('**/projects/review-race-project/cards/1/move', async (route) => {
    await new Promise((resolve) => setTimeout(resolve, 1200));
    await route.continue();
  });
  await page.route('**/projects/review-race-project/cards/*/comments', async (route) => {
    commentRequestURL = route.request().url();
    await route.continue();
  });

  await page.getByTestId('card-item').filter({ hasText: 'Race Card One' }).click();
  await expect(page.getByTestId('card-details-modal')).toBeVisible();
  await page.getByTestId('card-review-move-doing').click();
  await expect(page.getByTestId('review-reason-modal')).toBeVisible();
  await page.getByTestId('review-reason-input').fill('Race reason');
  await page.getByTestId('review-reason-submit').click();

  await page.getByTestId('card-item').filter({ hasText: 'Race Card Two' }).click();
  await expect(page.getByTestId('card-details-title')).toHaveText('Race Card Two');

  await expect.poll(() => commentRequestURL, { timeout: 10000 }).toContain('/projects/review-race-project/cards/1/comments');

  const cardOne = await fetch('http://127.0.0.1:18080/projects/review-race-project/cards/1');
  expect(cardOne.status).toBe(200);
  const cardOneBody = (await cardOne.json()) as { comments?: Array<{ body?: string }> };
  expect(cardOneBody.comments?.some((entry) => entry.body === 'Moved back to Doing: Race reason')).toBe(true);

  const cardTwo = await fetch('http://127.0.0.1:18080/projects/review-race-project/cards/2');
  expect(cardTwo.status).toBe(200);
  const cardTwoBody = (await cardTwo.json()) as { comments?: Array<{ body?: string }> };
  expect(cardTwoBody.comments?.length ?? 0).toBe(0);
});
