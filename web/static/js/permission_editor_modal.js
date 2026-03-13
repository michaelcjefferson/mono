/**
 * Permission Editor Modal — JS
 *
 * One modal is rendered per user card (hidden), opened from the card's
 * "Edit Permissions" button. State is tracked per-user-ID in permEditorState.
 *
 * State shape per user:
 *   originalPerms: string[]   — perms at page load (or last successful save)
 *   currentPerms:  string[]   — live working set (reflects UI)
 *
 * On save: POST /admin/users/:id/permissions
 *   body: { removePerms: string[], newPerms: string[] }
 * On error: revert UI to originalPerms.
 */

const permEditorState = {};

/**
 * Called once per user when the modal templ component renders.
 * Sets up initial state for that user.
 */
function initPermEditor(userId, originalPerms) {
  permEditorState[userId] = {
    originalPerms: [...originalPerms],
    currentPerms: [...originalPerms],
  };
}

// ── Open / Close ──────────────────────────────────────────────────────────────

function openPermModal(userId) {
  console.log(`open perm modal clicked - user ID ${userId}`)
  const backdrop = document.getElementById(`perm-modal-backdrop-${userId}`);
  if (!backdrop) {
    console.log("couldn't find modal backdrop element")
    return;
  }

  // Reset to last-known-good state before showing
  const state = permEditorState[userId];
  if (state) {
    renderPermState(userId, state.originalPerms);
    state.currentPerms = [...state.originalPerms];
  }

  hideErrorBanner(userId);
  hideSaveBtn(userId);

  backdrop.hidden = false;
  backdrop.removeAttribute('hidden');

  // Trap focus inside modal
  const modal = backdrop.querySelector('.perm-modal');
  trapFocus(modal);
}

function closePermModal(userId) {
  const backdrop = document.getElementById(`perm-modal-backdrop-${userId}`);
  if (backdrop) backdrop.hidden = true;
  restoreFocusAfterModal(userId);
}

function handlePermModalBackdropClick(event, backdrop) {
  // backdrop click (not modal panel) closes the modal
  if (event.target === backdrop) {
    const match = backdrop.id.match(/perm-modal-backdrop-(.+)/);
    if (match) closePermModal(match[1]);
  }
}

// Close on Escape key
document.addEventListener('keydown', function(e) {
  if (e.key !== 'Escape') return;
  const openBackdrop = document.querySelector('.perm-modal-backdrop:not([hidden])');
  if (!openBackdrop) return;
  const match = openBackdrop.id.match(/perm-modal-backdrop-(.+)/);
  if (match) closePermModal(match[1]);
});

// ── Permission manipulation ───────────────────────────────────────────────────

function handleAddPermission(btn) {
  const userId = btn.dataset.userId;
  const perm = btn.dataset.permission;
  const state = permEditorState[userId];
  if (!state || state.currentPerms.includes(perm)) return;

  state.currentPerms = [...state.currentPerms, perm];
  movePermPill(userId, perm, 'add');
  updateChangesBanner(userId);
  updateSaveBtn(userId);
  updateEmptyStates(userId);
}

function handleRemovePermission(btn) {
  const userId = btn.dataset.userId;
  const perm = btn.dataset.permission;
  const state = permEditorState[userId];
  if (!state || !state.currentPerms.includes(perm)) return;

  state.currentPerms = state.currentPerms.filter(p => p !== perm);
  movePermPill(userId, perm, 'remove');
  updateChangesBanner(userId);
  updateSaveBtn(userId);
  updateEmptyStates(userId);
}

/**
 * Move a pill between current and available sections by toggling its
 * CSS class and wiring the correct click handler.
 */
function movePermPill(userId, perm, direction) {
  const currentSection = document.getElementById(`perm-current-${userId}`);
  const availableSection = document.getElementById(`perm-available-${userId}`);

  if (direction === 'add') {
    // Find pill in available, move (visually) to current
    const sourcePill = availableSection.querySelector(`[data-permission="${perm}"]`);
    if (!sourcePill) return;

    sourcePill.hidden = true;

    // Build pill in current section
    const newPill = buildCurrentPill(userId, perm);
    currentSection.appendChild(newPill);

  } else {
    // Find pill in current, remove it; show in available
    const currentPill = currentSection.querySelector(`[data-permission="${perm}"]`);
    if (currentPill) currentPill.remove();

    const availPill = availableSection.querySelector(`[data-permission="${perm}"]`);
    if (availPill) availPill.hidden = false;
  }
}

function buildCurrentPill(userId, perm) {
  const parts = perm.split(':');
  const ns = parts[0] || perm;
  const action = parts[1] || '';

  const btn = document.createElement('button');
  btn.type = 'button';
  btn.className = `perm-pill perm-pill--current perm-pill--${ns} perm-pill--pending-add`;
  btn.dataset.permission = perm;
  btn.dataset.userId = userId;
  btn.title = `Remove ${perm}`;
  btn.onclick = function() { handleRemovePermission(this); };
  btn.setAttribute('aria-label', `Remove ${perm}`);

  btn.innerHTML = `
    <span class="perm-pill__ns">${escapeHtml(ns)}</span>
    <span class="perm-pill__sep">:</span>
    <span class="perm-pill__action">${escapeHtml(action)}</span>
    <span class="perm-pill__remove" aria-hidden="true">&#x2715;</span>
  `;
  return btn;
}

// ── UI state helpers ──────────────────────────────────────────────────────────

function updateEmptyStates(userId) {
  const currentSection = document.getElementById(`perm-current-${userId}`);
  const availableSection = document.getElementById(`perm-available-${userId}`);
  const currentEmpty = document.getElementById(`perm-current-empty-${userId}`);
  const availableEmpty = document.getElementById(`perm-available-empty-${userId}`);

  // Count actual pill buttons (not empty messages)
  const currentPills = currentSection.querySelectorAll('.perm-pill:not([hidden])').length;
  const availablePills = availableSection.querySelectorAll('.perm-pill:not([hidden])').length;

  if (currentEmpty) currentEmpty.hidden = currentPills > 0;
  if (availableEmpty) availableEmpty.hidden = availablePills > 0;
}

function updateChangesBanner(userId) {
  const state = permEditorState[userId];
  if (!state) return;

  const banner = document.getElementById(`perm-changes-banner-${userId}`);
  const text = document.getElementById(`perm-changes-text-${userId}`);
  if (!banner || !text) return;

  const { toAdd, toRemove } = computeDiff(state.originalPerms, state.currentPerms);
  const hasChanges = toAdd.length > 0 || toRemove.length > 0;

  banner.hidden = !hasChanges;
  if (hasChanges) {
    const parts = [];
    if (toAdd.length) parts.push(`+${toAdd.length} to add`);
    if (toRemove.length) parts.push(`-${toRemove.length} to remove`);
    text.textContent = `Pending: ${parts.join(', ')} — unsaved`;
  }
}

function updateSaveBtn(userId) {
  const state = permEditorState[userId];
  const btn = document.getElementById(`perm-save-btn-${userId}`);
  if (!state || !btn) return;

  const { toAdd, toRemove } = computeDiff(state.originalPerms, state.currentPerms);
  btn.disabled = toAdd.length === 0 && toRemove.length === 0;
}

function hideSaveBtn(userId) {
  const btn = document.getElementById(`perm-save-btn-${userId}`);
  if (btn) btn.disabled = true;
}

function showErrorBanner(userId, message) {
  const banner = document.getElementById(`perm-error-banner-${userId}`);
  const text = document.getElementById(`perm-error-text-${userId}`);
  if (banner) banner.hidden = false;
  if (text && message) text.textContent = message;
}

function hideErrorBanner(userId) {
  const banner = document.getElementById(`perm-error-banner-${userId}`);
  if (banner) banner.hidden = true;
}

// ── Save ──────────────────────────────────────────────────────────────────────

async function savePermissions(userId) {
  const state = permEditorState[userId];
  const saveBtn = document.getElementById(`perm-save-btn-${userId}`);
  if (!state || !saveBtn) return;

  const { toAdd, toRemove } = computeDiff(state.originalPerms, state.currentPerms);
  if (toAdd.length === 0 && toRemove.length === 0) return;

  // Loading state
  saveBtn.classList.add('is-loading');
  saveBtn.disabled = true;
  hideErrorBanner(userId);

  try {
    const response = await fetch(`/admin/users/${userId}/permissions`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        new_perms: toAdd,
        remove_perms: toRemove,
      }),
    });

    if (!response.ok) {
      let errMsg = 'Failed to update permissions. Changes have been reverted.';
      try {
        const errBody = await response.json();
        if (errBody.error) errMsg = errBody.error;
      } catch (_) {}
      throw new Error(errMsg);
    }

    // Success — commit new state as the baseline
    state.originalPerms = [...state.currentPerms];

    // Update the summary pill on the user card (if visible)
    updateCardPermissionSummary(userId, state.currentPerms);

    closePermModal(userId);

  } catch (err) {
    // Revert UI to originalPerms
    renderPermState(userId, state.originalPerms);
    state.currentPerms = [...state.originalPerms];
    updateChangesBanner(userId);
    updateSaveBtn(userId);
    showErrorBanner(userId, err.message);
  } finally {
    saveBtn.classList.remove('is-loading');
  }
}

// ── Render full state (used on open and on revert) ────────────────────────────

/**
 * Rebuilds the current-permissions section from scratch based on a permission array.
 * Resets available pills to their correct hidden/visible state.
 */
function renderPermState(userId, perms) {
  const currentSection = document.getElementById(`perm-current-${userId}`);
  const availableSection = document.getElementById(`perm-available-${userId}`);
  if (!currentSection || !availableSection) return;

  // Remove all dynamically added pills from current section
  // (leave the empty-state span)
  Array.from(currentSection.querySelectorAll('.perm-pill')).forEach(p => p.remove());

  // Reset available pills visibility
  Array.from(availableSection.querySelectorAll('.perm-pill')).forEach(pill => {
    const perm = pill.dataset.permission;
    pill.hidden = perms.includes(perm);
  });

  // Add current pills
  perms.forEach(perm => {
    const pill = buildCurrentPill(userId, perm);
    // Strip pending style — this is a confirmed state
    pill.classList.remove('perm-pill--pending-add');
    currentSection.appendChild(pill);
  });

  updateEmptyStates(userId);
}

// ── Sync card summary after save ─────────────────────────────────────────────

function updateCardPermissionSummary(userId, perms) {
  // Find the stat value element in the card for permissions
  const card = document.getElementById(`user-card-${userId}`);
  if (!card) return;
  const statValues = card.querySelectorAll('.user-card__stat-value');
  // First stat is permissions summary
  if (statValues[0]) {
    statValues[0].textContent = permissionSummaryFromArray(perms);
  }

  // Also refresh the badges in the expanded details if open
  const badgesContainer = card.querySelector('.user-card__badges');
  if (badgesContainer) {
    badgesContainer.innerHTML = '';
    if (perms.length === 0) {
      const empty = document.createElement('span');
      empty.className = 'user-card__empty';
      empty.textContent = 'No permissions assigned';
      badgesContainer.appendChild(empty);
    } else {
      perms.forEach(perm => {
        const parts = perm.split(':');
        const ns = parts[0] || perm;
        const span = document.createElement('span');
        span.className = `badge badge--${ns}`;
        span.textContent = perm;
        badgesContainer.appendChild(span);
      });
    }
  }
}

function permissionSummaryFromArray(perms) {
  let userCount = 0, adminCount = 0;
  perms.forEach(p => {
    if (p.startsWith('admin:')) adminCount++;
    else userCount++;
  });
  const parts = [];
  if (userCount) parts.push(`${userCount} user`);
  if (adminCount) parts.push(`${adminCount} admin`);
  return parts.length ? parts.join(' · ') : 'No permissions';
}

// ── Utility ───────────────────────────────────────────────────────────────────

function computeDiff(original, current) {
  const origSet = new Set(original);
  const currSet = new Set(current);
  return {
    toAdd:    current.filter(p => !origSet.has(p)),
    toRemove: original.filter(p => !currSet.has(p)),
  };
}

function escapeHtml(str) {
  return str
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#039;');
}

// ── Focus trapping (accessibility) ───────────────────────────────────────────

let _focusTrapCleanup = null;
let _lastFocusedElement = null;

function trapFocus(container) {
  _lastFocusedElement = document.activeElement;

  const focusable = container.querySelectorAll(
    'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])'
  );
  const first = focusable[0];
  const last = focusable[focusable.length - 1];

  function onKeyDown(e) {
    if (e.key !== 'Tab') return;
    if (e.shiftKey) {
      if (document.activeElement === first) {
        e.preventDefault();
        last && last.focus();
      }
    } else {
      if (document.activeElement === last) {
        e.preventDefault();
        first && first.focus();
      }
    }
  }

  container.addEventListener('keydown', onKeyDown);
  first && first.focus();

  if (_focusTrapCleanup) _focusTrapCleanup();
  _focusTrapCleanup = () => container.removeEventListener('keydown', onKeyDown);
}

function restoreFocusAfterModal(userId) {
  if (_focusTrapCleanup) {
    _focusTrapCleanup();
    _focusTrapCleanup = null;
  }
  if (_lastFocusedElement && typeof _lastFocusedElement.focus === 'function') {
    _lastFocusedElement.focus();
    _lastFocusedElement = null;
  }
}