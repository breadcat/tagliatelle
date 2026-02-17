// notes.js - Notes editor JavaScript

const editor = document.getElementById('editor');
const preview = document.getElementById('preview');
const searchInput = document.getElementById('search-input');

let originalContent = editor.value;
let currentContent = editor.value;

// Initialize preview with clickable links
document.addEventListener('DOMContentLoaded', () => {
    updatePreview(currentContent);
});

// Auto-update preview on typing (debounced)
let typingTimer;
editor.addEventListener('input', () => {
    clearTimeout(typingTimer);
    typingTimer = setTimeout(() => {
        currentContent = editor.value;
        updatePreview(currentContent);
    }, 500);
});

// Search functionality
searchInput.addEventListener('input', () => {
    const searchTerm = searchInput.value.toLowerCase();
    if (searchTerm === '') {
        updatePreview(currentContent);
        return;
    }

    const lines = currentContent.split('\n');
    const filtered = lines.filter(line =>
        line.toLowerCase().includes(searchTerm)
    );
    updatePreviewWithLinks(filtered.join('\n'));
});

// Convert URLs to clickable links and update preview
function updatePreview(content) {
    updatePreviewWithLinks(content);
}

function updatePreviewWithLinks(content) {
    // Clear preview
    preview.innerHTML = '';

    if (!content) return;

    const lines = content.split('\n');
    const urlRegex = /(https?:\/\/[^\s]+)/g;

    lines.forEach((line, index) => {
        const lineDiv = document.createElement('div');
        lineDiv.style.marginBottom = '0';

        // Check if line contains URLs
        if (urlRegex.test(line)) {
            // Reset regex lastIndex
            urlRegex.lastIndex = 0;

            let lastIndex = 0;
            let match;

            while ((match = urlRegex.exec(line)) !== null) {
                // Add text before URL
                if (match.index > lastIndex) {
                    const textNode = document.createTextNode(line.substring(lastIndex, match.index));
                    lineDiv.appendChild(textNode);
                }

                // Add clickable link
                const link = document.createElement('a');
                link.href = match[0];
                link.textContent = match[0];
                link.target = '_blank';
                link.rel = 'noopener noreferrer';
                // link.style.color = '#2563eb';
                link.style.textDecoration = 'underline';
                lineDiv.appendChild(link);

                lastIndex = match.index + match[0].length;
            }

            // Add remaining text after last URL
            if (lastIndex < line.length) {
                const textNode = document.createTextNode(line.substring(lastIndex));
                lineDiv.appendChild(textNode);
            }
        } else {
            // No URLs, just add plain text
            lineDiv.textContent = line;
        }

        preview.appendChild(lineDiv);
    });
}

function updateStats(stats) {
    document.getElementById('total-lines').textContent = stats.total_lines || 0;
    document.getElementById('categorized-lines').textContent = stats.categorized_lines || 0;
    document.getElementById('uncategorized-lines').textContent = stats.uncategorized || 0;
    document.getElementById('unique-categories').textContent = stats.unique_categories || 0;
}

function showMessage(text, type = 'success') {
    const msg = document.getElementById('message');
    msg.textContent = text;
    msg.className = `message ${type} show`;
    setTimeout(() => {
        msg.classList.remove('show');
    }, 3000);
}

async function saveNotes() {
    const content = editor.value;

    try {
        const response = await fetch('/notes/save', {
            method: 'POST',
            headers: {'Content-Type': 'application/x-www-form-urlencoded'},
            body: `content=${encodeURIComponent(content)}`
        });

        const result = await response.json();

        if (result.success) {
            showMessage('Notes saved successfully!', 'success');
            originalContent = content;
            // Reload to show sorted/deduped version
            setTimeout(() => location.reload(), 1000);
        } else {
            showMessage('Failed to save notes', 'error');
        }
    } catch (error) {
        showMessage('Error: ' + error.message, 'error');
    }
}

async function previewProcessing() {
    const content = editor.value;

    try {
        const response = await fetch('/notes/preview', {
            method: 'POST',
            headers: {'Content-Type': 'application/x-www-form-urlencoded'},
            body: `content=${encodeURIComponent(content)}`
        });

        const result = await response.json();

        if (result.success) {
            editor.value = result.content;
            currentContent = result.content;
            updatePreview(result.content);
            updateStats(result.stats);
            showMessage(`Processed: ${result.lineCount} lines after sort & dedupe`, 'success');
        }
    } catch (error) {
        showMessage('Error: ' + error.message, 'error');
    }
}

async function applySedRule(ruleIndex) {
    const content = editor.value;

    try {
        const response = await fetch('/notes/apply-sed', {
            method: 'POST',
            headers: {'Content-Type': 'application/x-www-form-urlencoded'},
            body: `content=${encodeURIComponent(content)}&rule_index=${ruleIndex}`
        });

        // Get the response text first to see what we're actually receiving
        const responseText = await response.text();
        console.log('Raw response:', responseText);
        console.log('Response status:', response.status);
        console.log('Content-Type:', response.headers.get('content-type'));

        // Try to parse as JSON
        let result;
        try {
            result = JSON.parse(responseText);
        } catch (parseError) {
            console.error('JSON parse error:', parseError);
            console.error('Response was:', responseText.substring(0, 200));
            showMessage('Server returned invalid response. Check console for details.', 'error');
            return;
        }

        if (result.success) {
            editor.value = result.content;
            currentContent = result.content;
            updatePreview(result.content);
            updateStats(result.stats);
            showMessage('Sed rule applied successfully!', 'success');
        } else {
            showMessage(result.error || 'Sed rule failed', 'error');
        }
    } catch (error) {
        console.error('Fetch error:', error);
        showMessage('Error: ' + error.message, 'error');
    }
}

function filterByCategory() {
    const category = document.getElementById('category-filter').value;
    if (!category) {
        updatePreview(currentContent);
        return;
    }

    const lines = currentContent.split('\n');

    // Handle uncategorized filter
    if (category === '__uncategorized__') {
        const filtered = lines.filter(line => {
            const trimmed = line.trim();
            if (!trimmed) return false;
            // Line is uncategorized if it doesn't contain '>' or '>' is not a separator
            return !trimmed.includes('>') || trimmed.indexOf('>') === trimmed.length - 1;
        });
        updatePreviewWithLinks(filtered.join('\n'));
        return;
    }

    // Handle normal category filter
    const filtered = lines.filter(line => {
        if (line.includes('>')) {
            const cat = line.split('>')[0].trim();
            return cat === category;
        }
        return false;
    });

    updatePreviewWithLinks(filtered.join('\n'));
}

function clearFilters() {
    searchInput.value = '';
    document.getElementById('category-filter').value = '';
    updatePreview(currentContent);
}

function exportNotes() {
    window.location.href = '/notes/export';
}

// Warn on unsaved changes
window.addEventListener('beforeunload', (e) => {
    if (editor.value !== originalContent) {
        e.preventDefault();
        e.returnValue = '';
    }
});
