import { formatVerseReference, parseVerseId } from './logic.js';

const timezone = Intl.DateTimeFormat().resolvedOptions().timeZone;

// If on login/register page, inject it into the form
const authForm = document.querySelector('.auth-form');
if (authForm) {
    const tzInput = document.createElement('input');
    tzInput.type = 'hidden';
    tzInput.name = 'timezone';
    tzInput.value = timezone;
    authForm.appendChild(tzInput);
}

// Get data from the page
const container = document.getElementById('content-container');
let currentDate = '';
let selectedVerseIds = [];

if (container && container.dataset.date) {
    currentDate = container.dataset.date;
    try {
        selectedVerseIds = JSON.parse(container.dataset.selectedVerses || '[]');
    } catch (e) {
        console.error('Failed to parse selected verses from container:', e);
    }
} else if (window.SOAP_DATA) {
    currentDate = window.SOAP_DATA.date || '';
    selectedVerseIds = window.SOAP_DATA.selectedVerses || [];
}

let saveTimeout = null;
const SAVE_DELAY = 1000; // 1 second after last change

// Get verse info from a verse element
function getVerseInfo(element) {
    // 1. Check for data-ref on the element itself or ancestors
    const refElement = element.closest('[data-ref]');
    if (refElement) {
        const ref = refElement.dataset.ref;
        return parseVerseId(ref);
    }

    // 2. Positional fallback: look for preceding verse number (only using .verse-num)
    const verseContent = element.closest('.verse-content');
    if (verseContent) {
        // Get all verse number elements in this container
        const allVerseNums = Array.from(verseContent.querySelectorAll('.verse-num'));

        if (allVerseNums.length > 0) {
            // Find the verse number that comes before this element
            let bestVerseNum = null;

            for (const verseNum of allVerseNums) {
                const position = element.compareDocumentPosition(verseNum);
                if (position & Node.DOCUMENT_POSITION_PRECEDING ||
                    position & Node.DOCUMENT_POSITION_CONTAINS) {
                    if (!bestVerseNum) {
                        bestVerseNum = verseNum;
                    } else {
                        const bestPos = bestVerseNum.compareDocumentPosition(verseNum);
                        if (bestPos & Node.DOCUMENT_POSITION_FOLLOWING) {
                            bestVerseNum = verseNum;
                        }
                    }
                }
            }

            if (bestVerseNum) {
                // Try to get info from the best verse number found
                // It should be a descendant of a [data-ref] span
                return getVerseInfo(bestVerseNum);
            }

            // Fallback: use the first verse number if nothing found
            const firstVerseNum = allVerseNums[0];
            if (firstVerseNum) {
                 return getVerseInfo(firstVerseNum);
            }
        }
    }

    return null;
}

// Update verse reference display
function updateVerseReference() {
    const selectedVersesReference = document.getElementById('selectedVersesReference');
    if (!selectedVersesReference) return;
    const reference = formatVerseReference(selectedVerseIds);
    if (reference) {
        selectedVersesReference.textContent = reference;
        selectedVersesReference.style.display = 'block';
    } else {
        selectedVersesReference.textContent = '';
        selectedVersesReference.style.display = 'none';
    }
}

// Toggle verse selection
function toggleVerseSelection(verseInfo) {
    if (!verseInfo) return;

    // Use the verse ID for consistency
    const baseId = verseInfo.id;
    const index = selectedVerseIds.findIndex(id => id === baseId);

    if (index > -1) {
        // Deselect
        selectedVerseIds.splice(index, 1);
        removeVerseHighlight(baseId);
    } else {
        // Select
        selectedVerseIds.push(verseInfo.id);
        highlightVerse(verseInfo.id);
    }

    updateVerseReference();
    scheduleSave();
}

// Highlight a verse
function highlightVerse(verseId) {
    // Select by data-ref
    const elements = document.querySelectorAll(`[data-ref="${verseId}"]`);
    elements.forEach(el => el.classList.add('verse-selected'));
}

// Remove verse highlight
function removeVerseHighlight(verseId) {
    const elements = document.querySelectorAll(`[data-ref="${verseId}"]`);
    elements.forEach(el => el.classList.remove('verse-selected'));
}

function refreshHighlights() {
    // Clear all
    document.querySelectorAll('.verse-selected').forEach(el => el.classList.remove('verse-selected'));

    // Highlight selected verses
    const uniqueIds = new Set();
    selectedVerseIds.forEach(verseId => {
        if (!uniqueIds.has(verseId)) {
            uniqueIds.add(verseId);
            highlightVerse(verseId);
        }
    });

    updateVerseReference();
}

function handleVerseClick(e) {
    // Only handle clicks within a verse inside the verses section
    if (!e.target.closest('.verses-section .verse-content')) {
        return;
    }

    // Prevent selection when clicking headers or extra_text
    if (e.target.closest('h1, h2, h3, h4, h5, h6, .extra_text')) {
        return;
    }

    const verseInfo = getVerseInfo(e.target);
    if (verseInfo) {
        e.preventDefault();
        toggleVerseSelection(verseInfo);
    }
}

function init() {
    refreshHighlights();
}

async function handleExportSubmit(e) {
    e.preventDefault();

    const exportForm = document.getElementById('export-form');
    const exportMethod = document.getElementById('export-method');
    const recipientsInput = document.getElementById('export-recipients');
    const exportModal = document.getElementById('export-modal');
    if (!exportForm || !exportMethod || !recipientsInput || !exportModal) return;

    const format = document.getElementById('export-format').value;
    const method = exportMethod.value;
    const recipients = recipientsInput.value.split(',').map(s => s.trim()).filter(s => s !== '');

    if (method === 'email' && recipients.length === 0) {
        alert('Please provide at least one recipient email.');
        return;
    }

    const submitBtn = exportForm.querySelector('button[type="submit"]');
    const originalBtnText = submitBtn?.textContent || 'Export';
    if (submitBtn) {
        submitBtn.disabled = true;
        submitBtn.textContent = 'Exporting...';
    }

    try {
        const response = await fetch('/export', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
                'X-CSRF-Token': window.SOAP_DATA?.csrfToken
            },
            body: JSON.stringify({
                date: currentDate,
                format: format,
                method: method,
                recipients: recipients
            })
        });

        if (!response.ok) {
            const errorData = await response.json().catch(() => ({}));
            throw new Error(errorData.error || `Server returned ${response.status}`);
        }

        if (method === 'email') {
            alert('SOAP entry has been queued for email delivery.');
            exportModal.close();
        } else {
            // Download handling
            const blob = await response.blob();
            const url = window.URL.createObjectURL(blob);
            const a = document.createElement('a');
            const contentDisposition = response.headers.get('Content-Disposition');
            let filename = `soap-${currentDate}.${format === 'markdown' ? 'md' : 'html'}`;

            if (contentDisposition && contentDisposition.includes('filename=')) {
                filename = contentDisposition.split('filename=')[1].split(';')[0].replace(/"/g, '').trim();
            }

            a.href = url;
            a.download = filename;
            document.body.appendChild(a);
            a.click();
            window.URL.revokeObjectURL(url);
            document.body.removeChild(a);
            exportModal.close();
        }
    } catch (err) {
        console.error('Export failed:', err);
        alert('Export failed: ' + err.message);
    } finally {
        if (submitBtn) {
            submitBtn.disabled = false;
            submitBtn.textContent = originalBtnText;
        }
    }
}

// Run initialization when DOM is ready
if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
} else {
    init();
}

// Listen for HTMX swaps to re-apply highlighting
document.body.addEventListener('htmx:afterSwap', function (evt) {
    if (evt.target.id === 'content-container' || evt.target.classList.contains('verses-section')) {
        const container = document.getElementById('content-container');
        if (container && container.dataset.date) {
            currentDate = container.dataset.date;
            try {
                selectedVerseIds = JSON.parse(container.dataset.selectedVerses || '[]');
            } catch (e) {
                console.error('Failed to parse selected verses from container:', e);
            }
            // Keep window.SOAP_DATA in sync
            if (window.SOAP_DATA) {
                window.SOAP_DATA.date = currentDate;
                window.SOAP_DATA.selectedVerses = selectedVerseIds;
            }
        } else if (window.SOAP_DATA) {
            currentDate = window.SOAP_DATA.date || '';
            selectedVerseIds = window.SOAP_DATA.selectedVerses || [];
        }
        refreshHighlights();
    }
});

// Configure HTMX to include CSRF token
document.body.addEventListener('htmx:configRequest', (event) => {
    if (window.SOAP_DATA?.csrfToken) {
        event.detail.headers['X-CSRF-Token'] = window.SOAP_DATA.csrfToken;
    }
});

// Body-level event delegation for clicks
document.body.addEventListener('click', function (e) {
    // Verse click handling
    handleVerseClick(e);

    // Share button click (opens export modal)
    const shareBtn = e.target.closest('#share-btn');
    if (shareBtn) {
        const exportModal = document.getElementById('export-modal');
        if (exportModal) {
            exportModal.showModal();
        }
        return;
    }

    // Close export modal button click
    const closeBtn = e.target.closest('#close-export-modal');
    if (closeBtn) {
        const exportModal = document.getElementById('export-modal');
        if (exportModal) {
            exportModal.close();
        }
        return;
    }

    // Export option card clicks
    const card = e.target.closest('.option-card');
    if (card) {
        const value = card.dataset.value;
        const targetId = card.dataset.target;
        const targetInput = document.getElementById(targetId);

        if (targetInput) {
            targetInput.value = value;

            // Update selected class
            const grid = card.closest('.option-grid');
            if (grid) {
                grid.querySelectorAll('.option-card').forEach(c => c.classList.remove('selected'));
            }
            card.classList.add('selected');

            // Trigger logic based on change
            if (targetId === 'export-method') {
                const recipientsGroup = document.getElementById('recipients-group');
                const recipientsInput = document.getElementById('export-recipients');
                if (value === 'email') {
                    if (recipientsGroup) recipientsGroup.style.display = 'block';
                    if (recipientsInput) recipientsInput.required = true;

                    // Hide Markdown format option
                    const formatMarkdown = document.getElementById('format-markdown');
                    if (formatMarkdown) {
                        formatMarkdown.style.display = 'none';

                        // If markdown was selected, switch to HTML
                        const exportFormat = document.getElementById('export-format');
                        if (exportFormat && exportFormat.value === 'markdown') {
                            exportFormat.value = 'html';
                            const htmlCard = document.querySelector('.option-card[data-value="html"][data-target="export-format"]');
                            if (htmlCard) {
                                const g = htmlCard.closest('.option-grid');
                                if (g) {
                                    g.querySelectorAll('.option-card').forEach(c => c.classList.remove('selected'));
                                }
                                htmlCard.classList.add('selected');
                            }
                        }
                    }
                } else {
                    if (recipientsGroup) recipientsGroup.style.display = 'none';
                    if (recipientsInput) recipientsInput.required = false;

                    // Show Markdown format option
                    const formatMarkdown = document.getElementById('format-markdown');
                    if (formatMarkdown) {
                        formatMarkdown.style.display = 'flex';
                    }
                }
            }
        }
    }
});

// Handle date changes using body-level event delegation
document.body.addEventListener('change', async function (e) {
    if (e.target.id === 'date-picker') {
        const datePicker = e.target;
        const newDate = datePicker.value;
        if (newDate === currentDate) return;

        // 1. Save data for the OLD date (currentDate)
        if (currentDate) {
            await saveData(true);
        }

        // 2. Trigger HTMX request
        datePicker.dispatchEvent(new CustomEvent('change-date'));
    }
});

// Handle export form submit using body-level event delegation
document.body.addEventListener('submit', function (e) {
    if (e.target.id === 'export-form') {
        handleExportSubmit(e);
    }
});

function saveData(immediate = false) {
    const observationField = document.getElementById('observation');
    const applicationField = document.getElementById('application');
    const prayerField = document.getElementById('prayer');
    const saveStatus = document.getElementById('saveStatus');

    // Guard against saving with empty date
    if (!currentDate || !observationField) {
        return Promise.resolve();
    }

    const dataToSave = {
        date: currentDate,
        observation: observationField.value,
        application: applicationField.value,
        prayer: prayerField.value,
        selectedVerses: selectedVerseIds
    };

    if (immediate) {
        if (saveTimeout) clearTimeout(saveTimeout);
    }

    if (saveStatus) {
        saveStatus.textContent = 'Saving...';
        saveStatus.className = 'save-status saving';
    }

    return fetch('/soap', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
            'X-CSRF-Token': window.SOAP_DATA?.csrfToken
        },
        body: JSON.stringify(dataToSave)
    })
        .then(response => response.json())
        .then(result => {
            if (result.error) {
                if (saveStatus) {
                    saveStatus.textContent = 'Error saving';
                    saveStatus.className = 'save-status error';
                }
            } else {
                if (saveStatus) {
                    saveStatus.textContent = 'Saved';
                    saveStatus.className = 'save-status saved';
                    setTimeout(() => {
                        // Only clear if status hasn't changed since
                        if (saveStatus.textContent === 'Saved') {
                            saveStatus.textContent = '';
                            saveStatus.className = 'save-status';
                        }
                    }, 2000);
                }
            }
        })
        .catch(error => {
            if (saveStatus) {
                saveStatus.textContent = 'Error saving';
                saveStatus.className = 'save-status error';
            }
            console.error('Error:', error);
        });
}

function scheduleSave() {
    if (saveTimeout) {
        clearTimeout(saveTimeout);
    }
    saveTimeout = setTimeout(saveData, SAVE_DELAY);
}

// Handle inputs using body-level event delegation
document.body.addEventListener('input', function (e) {
    if (e.target.id === 'observation' || e.target.id === 'application' || e.target.id === 'prayer') {
        scheduleSave();
    }
});
