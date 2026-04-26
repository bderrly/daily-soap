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

// Get data from the page (set by inline script in HTML)
let currentDate = window.SOAP_DATA?.date || '';
const observationField = document.getElementById('observation');
const applicationField = document.getElementById('application');
const prayerField = document.getElementById('prayer');
const saveStatus = document.getElementById('saveStatus');
const selectedVersesReference = document.getElementById('selectedVersesReference');
const datePicker = document.getElementById('date-picker');

// Export Modal Elements
const shareBtn = document.getElementById('share-btn');
const exportModal = document.getElementById('export-modal');
const closeExportModalBtn = document.getElementById('close-export-modal');
const exportForm = document.getElementById('export-form');
const exportMethod = document.getElementById('export-method');
const recipientsGroup = document.getElementById('recipients-group');
const recipientsInput = document.getElementById('export-recipients');

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

// Store selected verses as array of verse IDs
let selectedVerseIds = window.SOAP_DATA?.selectedVerses || [];

// Update verse reference display
function updateVerseReference() {
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
    // Delegate verse clicks to body to handle HTMX swaps
    document.body.addEventListener('click', handleVerseClick);
    refreshHighlights();

    // Export Modal listeners
    if (shareBtn && exportModal) {
        shareBtn.addEventListener('click', () => {
            exportModal.showModal();
        });
    }

    if (closeExportModalBtn && exportModal) {
        closeExportModalBtn.addEventListener('click', () => {
            exportModal.close();
        });
    }

    if (exportMethod) {
        exportMethod.addEventListener('change', () => {
            if (exportMethod.value === 'email') {
                recipientsGroup.style.display = 'block';
                recipientsInput.required = true;
            } else {
                recipientsGroup.style.display = 'none';
                recipientsInput.required = false;
            }
        });
    }

    if (exportForm) {
        exportForm.addEventListener('submit', handleExportSubmit);
    }
}

async function handleExportSubmit(e) {
    e.preventDefault();

    const format = document.getElementById('export-format').value;
    const method = exportMethod.value;
    const recipients = recipientsInput.value.split(',').map(s => s.trim()).filter(s => s !== '');

    if (method === 'email' && recipients.length === 0) {
        alert('Please provide at least one recipient email.');
        return;
    }

    const submitBtn = exportForm.querySelector('button[type="submit"]');
    const originalBtnText = submitBtn.textContent;
    submitBtn.disabled = true;
    submitBtn.textContent = 'Exporting...';

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
        submitBtn.disabled = false;
        submitBtn.textContent = originalBtnText;
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
    if (evt.target.classList.contains('verses-section')) {
        refreshHighlights();
    }
});

// Configure HTMX to include CSRF token
document.body.addEventListener('htmx:configRequest', (event) => {
    if (window.SOAP_DATA?.csrfToken) {
        event.detail.headers['X-CSRF-Token'] = window.SOAP_DATA.csrfToken;
    }
});

// Handle date changes
if (datePicker) {
    datePicker.addEventListener('change', function (e) {
        const newDate = datePicker.value;
        if (newDate === currentDate) return;

        // 1. Save data for the OLD date (currentDate)
        // Only save if we have a valid current date
        if (currentDate) {
            saveData(true);
        }

        // 2. Update current date
        currentDate = newDate;

        // 3. Load data for the NEW date
        loadDataForDate(newDate);
    });
}

function loadDataForDate(dateStr) {
    // Show loading state?
    if (observationField) observationField.value = 'Loading...';
    if (applicationField) applicationField.value = 'Loading...';
    if (prayerField) prayerField.value = 'Loading...';

    fetch(`/soap?date=${dateStr}`)
        .then(response => response.json())
        .then(data => {
            // Update fields
            if (observationField) observationField.value = data.observation || '';
            if (applicationField) applicationField.value = data.application || '';
            if (prayerField) prayerField.value = data.prayer || '';

            // Update selected verses
            selectedVerseIds = data.selectedVerses || [];

            // Update current date from server response (source of truth)
            if (data.date) {
                currentDate = data.date;
                // Ensure date picker reflects the actual date loaded
                if (datePicker && datePicker.value !== data.date) {
                    datePicker.value = data.date;
                }
            }

            // Refresh highlights
            refreshHighlights();
        })
        .catch(err => {
            console.error('Failed to load data', err);
            if (observationField) observationField.value = '';
            if (applicationField) applicationField.value = '';
            if (prayerField) prayerField.value = '';
        });
}

function saveData(immediate = false) {
    // Guard against saving with empty date
    if (!currentDate || !observationField) {
        return;
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

    fetch('/soap', {
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

if (observationField) observationField.addEventListener('input', scheduleSave);
if (applicationField) applicationField.addEventListener('input', scheduleSave);
if (prayerField) prayerField.addEventListener('input', scheduleSave);
