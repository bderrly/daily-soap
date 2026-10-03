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

// Try to get date from URL first
let urlDate = null;
try {
    if (window.location && window.location.search) {
        const urlParams = new URLSearchParams(window.location.search);
        urlDate = urlParams.get('date');
    }
} catch (e) {
    console.error('Failed to parse date from URL:', e);
}

if (urlDate) {
    currentDate = urlDate;
}

if (container && container.dataset.date) {
    if (!currentDate) {
        currentDate = container.dataset.date;
    }
    try {
        selectedVerseIds = JSON.parse(container.dataset.selectedVerses || '[]');
    } catch (e) {
        console.error('Failed to parse selected verses from container:', e);
    }
} else if (window.SOAP_DATA) {
    if (!currentDate) {
        currentDate = window.SOAP_DATA.date || '';
    }
    selectedVerseIds = window.SOAP_DATA.selectedVerses || [];
}

let saveTimeout = null;
const SAVE_DELAY = 1000; // 1 second after last change

// Get verse info from a verse element
function getVerseInfo(element) {
    const refElement = element.closest('.verses-section [data-ref]');
    if (refElement && refElement.dataset.ref) {
        return parseVerseId(refElement.dataset.ref);
    }

    return null;
}

// Normalize translation value to short-hand
function normalizeTranslationShorthand(t) {
    if (!t) return 'ESV';
    const upper = t.toUpperCase().trim();
    if (upper === 'NLT' || upper === 'D6E14A625393B4DA-01') return 'NLT';
    if (upper === 'MSG' || upper === '6F11A7DE016F942E-01' || upper === 'THE MESSAGE') return 'MSG';
    if (upper === 'ESV') return 'ESV';
    return upper;
}

// Get the active translation short-hand
function getActiveTranslation() {
    const translationSelect = document.getElementById('translation-select');
    if (translationSelect && translationSelect.value) {
        return normalizeTranslationShorthand(translationSelect.value);
    }
    const container = document.getElementById('content-container');
    if (container && container.dataset.translation) {
        return normalizeTranslationShorthand(container.dataset.translation);
    }
    if (window.SOAP_DATA && window.SOAP_DATA.translation) {
        return normalizeTranslationShorthand(window.SOAP_DATA.translation);
    }
    return 'ESV';
}

// Update verse reference display
function updateVerseReference() {
    const selectedVersesReference = document.getElementById('selectedVersesReference');
    if (!selectedVersesReference) return;
    const translation = getActiveTranslation();
    const reference = formatVerseReference(selectedVerseIds, translation);
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
    // Prevent selection if user is dragging to select text
    if (window.getSelection) {
        const selection = window.getSelection();
        if (selection && selection.toString().trim().length > 0) {
            return;
        }
    }

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

const cachedEntryDates = new Set();
const fetchedMonths = new Set();
let fpInstance = null;
let isChangingDate = false;

async function loadEntryDatesForMonth(monthStr, fp) {
    if (fetchedMonths.has(monthStr)) return;
    try {
        const res = await fetch(`/soap/dates?month=${encodeURIComponent(monthStr)}`);
        if (!res.ok) return;
        const data = await res.json();
        const dates = Array.isArray(data) ? data : (data.dates || []);
        dates.forEach(d => cachedEntryDates.add(d));
        fetchedMonths.add(monthStr);
        if (fp) {
            fp.redraw();
        }
    } catch (err) {
        console.error('Failed to load entry dates:', err);
    }
}

async function handleDateChange(newDate) {
    if (isChangingDate) return;
    if (!newDate || newDate === currentDate) return;

    isChangingDate = true;
    try {
        const datePicker = document.getElementById('date-picker');
        if (currentDate) {
            await saveData(true);
        }
        if (datePicker) {
            datePicker.value = newDate;
            datePicker.dispatchEvent(new CustomEvent('change-date'));
        }
    } finally {
        isChangingDate = false;
    }
}

function initDatePicker() {
    const datePicker = document.getElementById('date-picker');
    if (!datePicker) return;

    if (currentDate && datePicker.value !== currentDate) {
        datePicker.value = currentDate;
    }

    if (typeof window !== 'undefined' && typeof window.flatpickr === 'function') {
        if (fpInstance) {
            fpInstance.destroy();
            fpInstance = null;
        }

        fpInstance = window.flatpickr(datePicker, {
            defaultDate: currentDate || 'today',
            dateFormat: 'Y-m-d',
            minDate: '2026-01-01',
            allowInput: false,
            onDayCreate: function (dObj, dStr, fp, dayElem) {
                const y = dayElem.dateObj.getFullYear();
                const m = String(dayElem.dateObj.getMonth() + 1).padStart(2, '0');
                const d = String(dayElem.dateObj.getDate()).padStart(2, '0');
                const dateStr = `${y}-${m}-${d}`;
                if (cachedEntryDates.has(dateStr)) {
                    dayElem.classList.add('has-journal-entry');
                }
            },
            onMonthChange: async function (selectedDates, dateStr, instance) {
                const y = instance.currentYear;
                const m = String(instance.currentMonth + 1).padStart(2, '0');
                await loadEntryDatesForMonth(`${y}-${m}`, instance);
            },
            onYearChange: async function (selectedDates, dateStr, instance) {
                const y = instance.currentYear;
                const m = String(instance.currentMonth + 1).padStart(2, '0');
                await loadEntryDatesForMonth(`${y}-${m}`, instance);
            },
            onChange: async function (selectedDates, dateStr) {
                await handleDateChange(dateStr);
            }
        });

        const activeDate = currentDate || new Date().toISOString().substring(0, 10);
        const monthStr = activeDate.substring(0, 7);
        loadEntryDatesForMonth(monthStr, fpInstance);
    }
}

function init() {
    initDatePicker();
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
            if (container.dataset.translation && window.SOAP_DATA) {
                window.SOAP_DATA.translation = container.dataset.translation;
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
        initDatePicker();
    }
});

// Listen for HTMX settle to ensure verse reference display is preserved
document.body.addEventListener('htmx:afterSettle', function (evt) {
    if (evt.target.id === 'content-container' || evt.target.classList?.contains('verses-section')) {
        updateVerseReference();
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

// Handle date and translation changes using body-level event delegation
document.body.addEventListener('change', async function (e) {
    if (e.target.id === 'date-picker') {
        await handleDateChange(e.target.value);
    } else if (e.target.id === 'translation-select') {
        if (window.SOAP_DATA) {
            window.SOAP_DATA.translation = normalizeTranslationShorthand(e.target.value);
        }
        updateVerseReference();
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
        selectedVerses: selectedVerseIds,
        translation: getActiveTranslation()
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
                const hasContent = (observationField?.value.trim() || '') !== '' ||
                    (applicationField?.value.trim() || '') !== '' ||
                    (prayerField?.value.trim() || '') !== '' ||
                    selectedVerseIds.length > 0;
                if (currentDate) {
                    if (hasContent) {
                        cachedEntryDates.add(currentDate);
                    } else {
                        cachedEntryDates.delete(currentDate);
                    }
                    if (fpInstance) {
                        fpInstance.redraw();
                    }
                }

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
