
function initAjaxTagForms() {
  document.querySelectorAll('.ajax-tag-form').forEach((form) => {
    if (form.dataset.ajaxBound) return; // avoid double-binding after a swap
    form.dataset.ajaxBound = 'true';
    form.addEventListener('submit', handleAjaxTagSubmit);
  });
}

async function handleAjaxTagSubmit(e) {
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

    // Swap the tag list
    const newTagsList = doc.getElementById('tags-list');
    const currentTagsList = document.getElementById('tags-list');
    if (newTagsList && currentTagsList) {
      currentTagsList.innerHTML = newTagsList.innerHTML;
    }

    // Swap the category datalist (new categories may have been created)
    const newCategories = doc.getElementById('categories');
    const currentCategories = document.getElementById('categories');
    if (newCategories && currentCategories) {
      currentCategories.innerHTML = newCategories.innerHTML;
    }

    // Clear the "Add Tag" inputs after a submit
    const catInput = form.querySelector('input[name="category"]');
    const valInput = form.querySelector('input[name="value"]');
    if (catInput && valInput) {
      catInput.value = '';
      valInput.value = '';
      catInput.focus();
    }

    // Re-bind the delete buttons that just got swapped in
    initAjaxTagForms();
  } catch (err) {
    console.error('Tag update failed:', err);
  } finally {
    if (submitButton) submitButton.disabled = false;
  }
}

document.addEventListener('DOMContentLoaded', initAjaxTagForms);
