function toggleDescriptionEdit() {
    const displayDiv = document.getElementById('description-display');
    const editDiv = document.getElementById('description-edit');

    displayDiv.style.display = 'none';
    editDiv.style.display = 'block';

    // Focus the textarea and update character count
    const textarea = document.getElementById('description-textarea');
    textarea.focus();

    // Move cursor to end of text if there's existing content
    if (textarea.value) {
        textarea.setSelectionRange(textarea.value.length, textarea.value.length);
    }
}

function cancelDescriptionEdit() {
    const displayDiv = document.getElementById('description-display');
    const editDiv = document.getElementById('description-edit');
    const textarea = document.getElementById('description-textarea');

    // Reset textarea to original value
    const original = displayDiv.dataset.originalDescription || '';
    textarea.value = original;

    displayDiv.style.display = 'block';
    editDiv.style.display = 'none';

    // Re-run conversion so any [file/123] becomes clickable again
    convertFileRefs();
}

// Auto-resize textarea as content changes
document.addEventListener('DOMContentLoaded', function() {
    const textarea = document.getElementById('description-textarea');
    if (textarea) {
        textarea.addEventListener('input', function() {
            // Reset height to auto to get the correct scrollHeight
            this.style.height = 'auto';
            // Set the height to match the content, with a minimum of 6 rows
            const minHeight = parseInt(getComputedStyle(this).lineHeight) * 6;
            this.style.height = Math.max(minHeight, this.scrollHeight) + 'px';
        });
    }
});

// Allow [file/123] and [/file/123] links to become clickable
function convertFileRefs() {
  const el = document.getElementById("current-description");
  if (!el) return;

  const pattern = /\[\/?file\/(\d+)\]/g;

  // Walk through text nodes only, preserving existing HTML elements
  function processTextNodes(node) {
    if (node.nodeType === Node.TEXT_NODE) {
      let text = node.textContent;

      // Check if this text node contains file references
      if (pattern.test(text)) {
        // Reset regex lastIndex
        pattern.lastIndex = 0;

        // Replace file references
        text = text.replace(pattern, (_, id) => {
          return `<a href="/file/${id}" class="file-link">file/${id}</a>`;
        });

        // Create a temporary container and replace the text node
        const temp = document.createElement('span');
        temp.innerHTML = text;
        const parent = node.parentNode;
        while (temp.firstChild) {
          parent.insertBefore(temp.firstChild, node);
        }
        parent.removeChild(node);
      }
    } else if (node.nodeType === Node.ELEMENT_NODE && node.tagName !== 'A') {
      // Don't process inside existing anchor tags
      Array.from(node.childNodes).forEach(processTextNodes);
    }
  }

  processTextNodes(el);
}

// Re-init clickable links after submit
function reinitDescriptionEnhancements() {
  const hasCurrentDescription = !!document.getElementById('current-description');

  if (hasCurrentDescription && typeof makeTimestampsClickable === 'function') {
    makeTimestampsClickable('current-description', 'videoPlayer', 'imageViewer');
  }

  if (hasCurrentDescription && typeof makeLineNumbersClickable === 'function') {
    makeLineNumbersClickable('current-description', 'text-viewer');
  }

  convertFileRefs();
}

// AJAX-submit instead of usual reload
function initDescriptionForm() {
  const editDiv = document.getElementById('description-edit');
  if (!editDiv) return;

  const form = editDiv.querySelector('form');
  if (!form || form.dataset.ajaxBound) return; // avoid double-binding
  form.dataset.ajaxBound = 'true';
  form.addEventListener('submit', handleDescriptionSubmit);
}

async function handleDescriptionSubmit(e) {
  e.preventDefault();
  const form = e.target;
  const submitButton = form.querySelector('button[type="submit"]');
  if (submitButton) submitButton.disabled = true;

  const action = form.getAttribute('action') || window.location.pathname;

  try {
    const response = await fetch(action, {
      method: 'POST',
      body: new FormData(form),
    });

    if (!response.ok) {
      throw new Error('Request failed with status ' + response.status);
    }

    const html = await response.text();
    const doc = new DOMParser().parseFromString(html, 'text/html');

    const newDisplay = doc.getElementById('description-display');
    const currentDisplay = document.getElementById('description-display');
    if (newDisplay && currentDisplay) {
      currentDisplay.innerHTML = newDisplay.innerHTML;
      currentDisplay.dataset.originalDescription = newDisplay.dataset.originalDescription || '';
    }

    const newTextarea = doc.getElementById('description-textarea');
    const currentTextarea = document.getElementById('description-textarea');
    if (newTextarea && currentTextarea) {
      currentTextarea.value = newTextarea.value;
    }

    document.getElementById('description-display').style.display = 'block';
    document.getElementById('description-edit').style.display = 'none';

    // Re-run scripts against new description
    reinitDescriptionEnhancements();
  } catch (err) {
    console.error('Description update failed:', err);
    alert('Failed to save description. Please try again.');
  } finally {
    if (submitButton) submitButton.disabled = false;
  }
}

document.addEventListener("DOMContentLoaded", function() {
  convertFileRefs();
  initDescriptionForm();
});