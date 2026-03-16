document.addEventListener('DOMContentLoaded', function () {
  const fileRangeInput = document.getElementById('file_range');
  if (!fileRangeInput) return;
  const fileForm = fileRangeInput.closest('form');
  if (!fileForm) return;

  function updateValueField() {
    const checkedOp = fileForm.querySelector('input[name="operation"]:checked');
    const valueField = fileForm.querySelector('#value');
    const valueLabel = fileForm.querySelector('label[for="value"]');
    if (!checkedOp || !valueField || !valueLabel) return;
    if (checkedOp.value === 'add') {
        valueField.required = true;
        valueLabel.innerHTML = 'Value <span class="required">(required)</span>:';
    } else {
        valueField.required = false;
        valueLabel.innerHTML = 'Value:';
    }
  }

  function toggleSelectionMode() {
    const rangeMode = fileForm.querySelector('input[name="selection_mode"][value="range"]');
    if (!rangeMode) return;

    const isRangeMode = rangeMode.checked;
    const rangeSelection = document.getElementById('range-selection');
    const tagSelection = document.getElementById('tag-selection');
    const fileRangeField = document.getElementById('file_range');
    const tagQueryField = document.getElementById('tag_query');

    if (isRangeMode && tagQueryField && fileRangeField && !fileRangeField.value && tagQueryField.value) {
      fileRangeField.value = tagQueryField.value;
    } else if (!isRangeMode && fileRangeField && tagQueryField && !tagQueryField.value && fileRangeField.value) {
      tagQueryField.value = fileRangeField.value;
    }

    if (rangeSelection) rangeSelection.style.display = isRangeMode ? 'block' : 'none';
    if (tagSelection) tagSelection.style.display = isRangeMode ? 'none' : 'block';

    // Update required attributes
    if (fileRangeField) fileRangeField.required = isRangeMode;
    if (tagQueryField) tagQueryField.required = !isRangeMode;

    if (isRangeMode && fileRangeField) {
      fileRangeField.focus();
    } else if (!isRangeMode && tagQueryField) {
      tagQueryField.focus();
    }
  }

  // Set up event listeners for operation radio buttons
  fileForm.querySelectorAll('input[name="operation"]').forEach(function (radio) {
    radio.addEventListener('change', updateValueField);
  });

  // Set up event listeners for selection mode radio buttons
  fileForm.querySelectorAll('input[name="selection_mode"]').forEach(function (radio) {
    radio.addEventListener('change', toggleSelectionMode);
  });

  // Initialize on page load
  updateValueField();
  toggleSelectionMode();

  // Add form validation with selection mode awareness
  fileForm.addEventListener('submit', function (e) {
    const selectionModeRadio = fileForm.querySelector('input[name="selection_mode"]:checked');
    const selectionMode = selectionModeRadio ? selectionModeRadio.value : 'range';

    const fileRange = (fileForm.querySelector('#file_range') || { value: '' }).value.trim();
    const tagQuery = (fileForm.querySelector('#tag_query') || { value: '' }).value.trim();
    const category = (fileForm.querySelector('#category') || { value: '' }).value.trim();
    const value = (fileForm.querySelector('#value') || { value: '' }).value.trim();
    const checkedOp = fileForm.querySelector('input[name="operation"]:checked');
    const operation = checkedOp ? checkedOp.value : '';

    // Validate based on selection mode
    if (selectionMode === 'range') {
      if (!fileRange) {
        alert('Please enter a file ID range');
        e.preventDefault();
        return;
      }
      const rangePattern = /^[\d\s,-]+$/;
      if (!rangePattern.test(fileRange)) {
        alert('File range should only contain numbers, commas, dashes, and spaces');
        e.preventDefault();
        return;
      }
    } else if (selectionMode === 'tags') {
      if (!tagQuery) {
        alert('Please enter a tag query');
        e.preventDefault();
        return;
      }
      // Basic validation for tag query format
      const tagPattern = /^[^:]+:[^:]+(\s+OR\s+[^:]+:[^:]+|,[^:]+:[^:]+)*$/i;
      if (!tagPattern.test(tagQuery)) {
        alert('Tag query format should be "category:value" (e.g., "colour:blue" or "colour:blue,size:large")');
        e.preventDefault();
        return;
      }
    }

    if (!category) {
      alert('Please enter a category');
      e.preventDefault();
      return;
    }

    if (operation === 'add' && !value) {
      alert('Please enter a tag value when adding tags');
      e.preventDefault();
      return;
    }
  });

  // Hover previews for recent images
  const IMAGE_EXTENSIONS = /\.(jpe?g|png|gif|webp)$/i;

  const tooltip = document.createElement('div');
  tooltip.id = 'thumb-tooltip';
  Object.assign(tooltip.style, {
    position: 'fixed',
    display: 'none',
    pointerEvents: 'none',
    zIndex: '9999',
    border: '1px solid #888',
    background: '#fff',
    padding: '3px',
    borderRadius: '4px',
    boxShadow: '0 2px 8px rgba(0,0,0,0.3)',
  });
  const tooltipImg = document.createElement('img');
  tooltipImg.style.maxWidth = '200px';
  tooltipImg.style.maxHeight = '200px';
  tooltipImg.style.display = 'block';
  tooltip.appendChild(tooltipImg);
  document.body.appendChild(tooltip);

  document.querySelectorAll('.file-item a').forEach(function (link) {
    const filename = link.title || link.textContent.trim();
    if (!IMAGE_EXTENSIONS.test(filename)) return;

    const thumbUrl = '/uploads/' + filename;

    link.addEventListener('mouseenter', function () {
      tooltipImg.src = thumbUrl;
      tooltip.style.display = 'block';
    });

    link.addEventListener('mousemove', function (e) {
      const pad = 16;
      const tw = tooltip.offsetWidth;
      const th = tooltip.offsetHeight;
      let x = e.clientX + pad;
      let y = e.clientY + pad;
      if (x + tw > window.innerWidth)  x = e.clientX - tw - pad;
      if (y + th > window.innerHeight) y = e.clientY - th - pad;
      tooltip.style.left = x + 'px';
      tooltip.style.top  = y + 'px';
    });

    link.addEventListener('mouseleave', function () {
      tooltip.style.display = 'none';
      tooltipImg.src = '';
    });
  });
});
