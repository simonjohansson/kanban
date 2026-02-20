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

  await page.reload();

  await page.getByTestId('project-item').filter({ hasText: 'Modal Project' }).click();
  await expect(page.getByTestId('lane-Todo')).toContainText('Card One');
  await expect(page.getByTestId('lane-Todo')).toContainText('Card Two');

  await page.getByTestId('card-item').filter({ hasText: 'Card One' }).click();
  await expect(page.getByTestId('card-details-modal')).toBeVisible();
  await expect(page.getByTestId('card-details-title')).toHaveText('Card One');
  await expect(page.getByTestId('card-details-branch')).toContainText('feature/card-one');
  await expect(page.getByTestId('card-details-description')).toContainText('First description body');
  await expect(page.getByTestId('card-details-comments')).toContainText('First comment line 1');
  await expect(page.getByTestId('card-details-comments')).toContainText('First comment line 2');
  await expect(page.getByTestId('card-details-comments')).not.toContainText('\\n');
  await expect(page).toHaveURL(/\/card\/modal-project\/1$/);

  await page.getByTestId('card-item').filter({ hasText: 'Card Two' }).click();
  await expect(page.getByTestId('card-details-title')).toHaveText('Card Two');
  await expect(page.getByTestId('card-details-branch')).toContainText('feature/card-two');
  await expect(page.getByTestId('card-details-description')).toContainText('Second description body');
  await expect(page.getByTestId('card-details-comments')).toContainText('Second comment body');
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
