<script lang="ts">
  import { onDestroy, onMount } from 'svelte';
  import type { Card } from '../generated';

  export let card: Card | null = null;
  export let loading = false;
  export let errorMessage: string | null = null;
  export let onClose: () => void = () => {};
  export let onRetry: () => void = () => {};
  export let reviewActionsVisible = false;
  export let reviewActionBusy = false;
  export let onMoveReviewToTodo: () => void = () => {};
  export let onMoveReviewToDoing: () => void = () => {};
  export let onMoveReviewToDone: () => void = () => {};
  export let reviewReasonTargetStatus: 'Todo' | 'Doing' | null = null;
  export let reviewReasonInput = '';
  export let reviewReasonError: string | null = null;
  export let onReviewReasonInput: (value: string) => void = () => {};
  export let onSubmitReviewReason: () => void = () => {};
  export let onCancelReviewReason: () => void = () => {};

  let modalElement: HTMLElement | null = null;

  onMount(() => {
    const handlePointerDown = (event: PointerEvent): void => {
      if (!(event.target instanceof Node)) {
        return;
      }
      if (modalElement?.contains(event.target)) {
        return;
      }
      onClose();
    };
    window.addEventListener('pointerdown', handlePointerDown);
    return () => {
      window.removeEventListener('pointerdown', handlePointerDown);
    };
  });

  onDestroy(() => {
    modalElement = null;
  });

  function renderText(body: string): string {
    return body.replace(/\\n/g, '\n');
  }
</script>

<div class="layer">
  <section bind:this={modalElement} class="modal" data-testid="card-details-modal" on:click|stopPropagation>
    <header class="header">
      <h2 data-testid="card-details-title">{card?.title ?? 'Card details'}</h2>
      <button data-testid="card-details-close" on:click={onClose} type="button">Close</button>
    </header>

    {#if loading}
      <p class="state" data-testid="card-details-loading">Loading card details...</p>
    {:else if errorMessage}
      <div class="state">
        <p data-testid="card-details-error">{errorMessage}</p>
        <button data-testid="card-details-retry" on:click={onRetry} type="button">Retry</button>
      </div>
    {:else if card}
      <div class="details-grid">
        <section>
          <h3>Branch</h3>
          <p class="mono" data-testid="card-details-branch">{card.branch?.trim() || 'No branch'}</p>
        </section>

        <section>
          <h3>Description</h3>
          <div data-testid="card-details-description">
            {#if !card.description || card.description.length === 0}
              <p class="empty">No description</p>
            {:else}
              {#each card.description ?? [] as entry}
                <article class="event">
                  <p>{renderText(entry.body)}</p>
                </article>
              {/each}
            {/if}
          </div>
        </section>

        <section>
          <h3>Comments</h3>
          <div data-testid="card-details-comments">
            {#if !card.comments || card.comments.length === 0}
              <p class="empty">No comments</p>
            {:else}
              {#each card.comments ?? [] as entry}
                <article class="event">
                  <p>{renderText(entry.body)}</p>
                </article>
              {/each}
            {/if}
          </div>
        </section>

        {#if reviewActionsVisible}
          <section class="review-actions" data-testid="card-review-actions">
            <h3>Review Actions</h3>
            <div class="review-actions-buttons">
              <button
                data-testid="card-review-move-todo"
                disabled={reviewActionBusy}
                on:click={onMoveReviewToTodo}
                type="button"
              >
                Move to Todo
              </button>
              <button
                data-testid="card-review-move-doing"
                disabled={reviewActionBusy}
                on:click={onMoveReviewToDoing}
                type="button"
              >
                Move to Doing
              </button>
              <button
                data-testid="card-review-move-done"
                disabled={reviewActionBusy}
                on:click={onMoveReviewToDone}
                type="button"
              >
                Move to Done
              </button>
            </div>
          </section>
        {/if}

        {#if reviewReasonTargetStatus}
          <section class="reason-modal" data-testid="review-reason-modal">
            <h3>Reason for Moving to {reviewReasonTargetStatus}</h3>
            <textarea
              data-testid="review-reason-input"
              on:input={(event) => onReviewReasonInput((event.currentTarget as HTMLTextAreaElement).value)}
              placeholder="Why is this card moving back?"
              rows="4"
              value={reviewReasonInput}
            />
            {#if reviewReasonError}
              <p class="reason-error" data-testid="review-reason-error">{reviewReasonError}</p>
            {/if}
            <div class="reason-modal-buttons">
              <button
                data-testid="review-reason-submit"
                disabled={reviewActionBusy}
                on:click={onSubmitReviewReason}
                type="button"
              >
                Confirm
              </button>
              <button
                data-testid="review-reason-cancel"
                disabled={reviewActionBusy}
                on:click={onCancelReviewReason}
                type="button"
              >
                Cancel
              </button>
            </div>
          </section>
        {/if}
      </div>
    {/if}
  </section>
</div>

<style>
  .layer {
    position: fixed;
    inset: 0;
    display: grid;
    place-items: center;
    padding: 20px;
    z-index: 40;
    pointer-events: none;
  }

  .modal {
    width: min(760px, 100%);
    max-height: min(82vh, 900px);
    overflow: auto;
    background: #ffffff;
    border: 1px solid #d1d5db;
    border-radius: 12px;
    box-shadow: 0 20px 40px rgba(15, 23, 42, 0.22);
    padding: 16px;
    pointer-events: auto;
  }

  .header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 10px;
    margin-bottom: 14px;
  }

  .header h2 {
    margin: 0;
    font-size: 1.08rem;
    color: #111827;
  }

  .header button {
    border: 1px solid #9ca3af;
    background: #f9fafb;
    border-radius: 8px;
    padding: 7px 10px;
    cursor: pointer;
  }

  .details-grid {
    display: grid;
    gap: 14px;
  }

  h3 {
    margin: 0 0 8px;
    font-size: 0.9rem;
    color: #374151;
  }

  .mono {
    margin: 0;
    font-size: 0.82rem;
    font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, Liberation Mono, Courier New, monospace;
    color: #1f2937;
    overflow-wrap: anywhere;
  }

  .event {
    border: 1px solid #e5e7eb;
    border-radius: 8px;
    padding: 8px;
    background: #f9fafb;
  }

  .event p {
    margin: 0;
    color: #111827;
    font-size: 0.9rem;
    white-space: pre-wrap;
  }

  .empty {
    margin: 0;
    color: #6b7280;
    font-size: 0.9rem;
  }

  .state {
    display: grid;
    gap: 10px;
    color: #111827;
  }

  .state button {
    justify-self: start;
    border: 1px solid #0f766e;
    background: #0f766e;
    color: #fff;
    border-radius: 8px;
    padding: 8px 10px;
    cursor: pointer;
  }

  .review-actions {
    border-top: 1px solid #e5e7eb;
    padding-top: 6px;
  }

  .review-actions-buttons {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
  }

  .review-actions-buttons button {
    border: 1px solid #0f766e;
    background: #0f766e;
    color: #fff;
    border-radius: 8px;
    padding: 8px 10px;
    cursor: pointer;
  }

  .review-actions-buttons button:disabled {
    opacity: 0.65;
    cursor: default;
  }

  .reason-modal {
    border: 1px solid #e5e7eb;
    border-radius: 10px;
    padding: 10px;
    background: #f9fafb;
    display: grid;
    gap: 8px;
  }

  .reason-modal textarea {
    width: 100%;
    box-sizing: border-box;
    font: inherit;
    border: 1px solid #d1d5db;
    border-radius: 8px;
    padding: 8px;
    resize: vertical;
  }

  .reason-modal-buttons {
    display: flex;
    gap: 8px;
  }

  .reason-modal-buttons button {
    border: 1px solid #0f766e;
    background: #0f766e;
    color: #fff;
    border-radius: 8px;
    padding: 8px 10px;
    cursor: pointer;
  }

  .reason-modal-buttons button:disabled {
    opacity: 0.65;
    cursor: default;
  }

  .reason-error {
    margin: 0;
    color: #991b1b;
    font-size: 0.9rem;
  }
</style>
