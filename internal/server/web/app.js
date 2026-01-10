(function () {
    // Get data from the page (set by inline script in HTML)
    let currentDate = window.SOAP_DATA?.date || '';
    const observationField = document.getElementById('observation');
    const applicationField = document.getElementById('application');
    const prayerField = document.getElementById('prayer');
    const saveStatus = document.getElementById('saveStatus');
    const selectedVersesReference = document.getElementById('selectedVersesReference');
    const datePicker = document.getElementById('date-picker');

    let saveTimeout = null;
    const SAVE_DELAY = 1000; // 1 second after last change

    // Book name mapping (ESV Bible order, 1-indexed, so book 1 = Genesis, book 23 = Isaiah)
    const bookNames = {
        1: 'Genesis', 2: 'Exodus', 3: 'Leviticus', 4: 'Numbers', 5: 'Deuteronomy',
        6: 'Joshua', 7: 'Judges', 8: 'Ruth', 9: '1 Samuel', 10: '2 Samuel',
        11: '1 Kings', 12: '2 Kings', 13: '1 Chronicles', 14: '2 Chronicles', 15: 'Ezra',
        16: 'Nehemiah', 17: 'Esther', 18: 'Job', 19: 'Psalm', 20: 'Proverbs',
        21: 'Ecclesiastes', 22: 'Song of Solomon', 23: 'Isaiah', 24: 'Jeremiah', 25: 'Lamentations',
        26: 'Ezekiel', 27: 'Daniel', 28: 'Hosea', 29: 'Joel', 30: 'Amos',
        31: 'Obadiah', 32: 'Jonah', 33: 'Micah', 34: 'Nahum', 35: 'Habakkuk',
        36: 'Zephaniah', 37: 'Haggai', 38: 'Zechariah', 39: 'Malachi', 40: 'Matthew',
        41: 'Mark', 42: 'Luke', 43: 'John', 44: 'Acts', 45: 'Romans',
        46: '1 Corinthians', 47: '2 Corinthians', 48: 'Galatians', 49: 'Ephesians', 50: 'Philippians',
        51: 'Colossians', 52: '1 Thessalonians', 53: '2 Thessalonians', 54: '1 Timothy', 55: '2 Timothy',
        56: 'Titus', 57: 'Philemon', 58: 'Hebrews', 59: 'James', 60: '1 Peter',
        61: '2 Peter', 62: '1 John', 63: '2 John', 64: '3 John', 65: 'Jude', 66: 'Revelation'
    };

    // Parse verse ID (format: v23063008-1 where 23=book, 063=chapter, 008=verse)
    // Parse verse ID (format: 23063008)
    function parseVerseId(verseId) {
        // Strict format: 8 digits
        const match = verseId.match(/^(\d{2})(\d{3})(\d{3})$/);
        if (!match) return null;
        return {
            book: parseInt(match[1], 10),
            chapter: parseInt(match[2], 10),
            verse: parseInt(match[3], 10),
            id: verseId
        };
    }

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

    // Format selected verses as reference string (e.g., "Isaiah 63:8-9")
    function formatVerseReference(verseIds) {
        if (verseIds.length === 0) return '';

        // Group verses by book and chapter
        const groups = {};
        for (const verseId of verseIds) {
            const baseId = getBaseVerseId(verseId);
            const info = parseVerseId(baseId);
            if (!info) continue;
            const key = `${info.book}-${info.chapter}`;
            if (!groups[key]) {
                groups[key] = {
                    book: info.book,
                    chapter: info.chapter,
                    verses: []
                };
            }
            if (!groups[key].verses.includes(info.verse)) {
                groups[key].verses.push(info.verse);
            }
        }

        // Format each group
        const references = [];
        for (const key in groups) {
            const group = groups[key];
            group.verses.sort((a, b) => a - b);
            const bookName = bookNames[group.book] || `Book ${group.book}`;

            // Combine contiguous verses into ranges
            const ranges = [];
            let rangeStart = group.verses[0];
            let rangeEnd = group.verses[0];

            for (let i = 1; i < group.verses.length; i++) {
                if (group.verses[i] === rangeEnd + 1) {
                    rangeEnd = group.verses[i];
                } else {
                    if (rangeStart === rangeEnd) {
                        ranges.push(rangeStart.toString());
                    } else {
                        ranges.push(`${rangeStart}-${rangeEnd}`);
                    }
                    rangeStart = group.verses[i];
                    rangeEnd = group.verses[i];
                }
            }
            // Add the last range
            if (rangeStart === rangeEnd) {
                ranges.push(rangeStart.toString());
            } else {
                ranges.push(`${rangeStart}-${rangeEnd}`);
            }

            references.push(`${bookName} ${group.chapter}:${ranges.join(',')}`);
        }

        return references.join('; ');
    }

    // Update verse reference display
    function updateVerseReference() {
        const reference = formatVerseReference(selectedVerseIds);
        if (reference) {
            selectedVersesReference.textContent = reference;
            selectedVersesReference.style.display = 'block';
        } else {
            selectedVersesReference.textContent = '';
            selectedVersesReference.style.display = 'none';
        }
    }

    // Get the base verse ID (8 digits)
    function getBaseVerseId(verseId) {
        return verseId; // Already simple digits in this system
    }

    // Toggle verse selection
    function toggleVerseSelection(verseInfo) {
        if (!verseInfo) return;

        // Use the base verse ID (without suffix) for consistency
        const baseId = getBaseVerseId(verseInfo.id);
        const index = selectedVerseIds.findIndex(id => getBaseVerseId(id) === baseId);

        if (index > -1) {
            // Deselect
            selectedVerseIds.splice(index, 1);
            removeVerseHighlight(baseId);
        } else {
            // Select - ensure we use the base ID (digits only) or whatever format we prefer
            // Currently using the returned ID which might be just digits now
            selectedVerseIds.push(verseInfo.id);
            highlightVerse(verseInfo.id);
        }

        updateVerseReference();
        scheduleSave();
    }

    // Highlight a verse
    function highlightVerse(verseId) {
        const baseId = getBaseVerseId(verseId); // Get 8 digit ref
        // Select by data-ref
        const elements = document.querySelectorAll(`[data-ref="${baseId}"]`);
        elements.forEach(el => el.classList.add('verse-selected'));
    }

    // Remove verse highlight
    function removeVerseHighlight(verseId) {
        const baseId = getBaseVerseId(verseId);
        const elements = document.querySelectorAll(`[data-ref="${baseId}"]`);
        elements.forEach(el => el.classList.remove('verse-selected'));
    }


    function refreshHighlights() {
        // Clear all
        document.querySelectorAll('.verse-selected').forEach(el => el.classList.remove('verse-selected'));

        // Highlight selected verses
        const uniqueBaseIds = new Set();
        selectedVerseIds.forEach(verseId => {
            const baseId = getBaseVerseId(verseId);
            if (!uniqueBaseIds.has(baseId)) {
                uniqueBaseIds.add(baseId);
                highlightVerse(verseId);
            }
        });

        updateVerseReference();
    }

    function handleVerseClick(e) {
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
        const versesSection = document.querySelector('.verses-section');
        if (versesSection) {
            versesSection.addEventListener('click', handleVerseClick);
        }
        refreshHighlights();
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
        observationField.value = 'Loading...';
        applicationField.value = 'Loading...';
        prayerField.value = 'Loading...';

        fetch(`/soap?date=${dateStr}`)
            .then(response => response.json())
            .then(data => {
                // Update fields
                observationField.value = data.observation || '';
                applicationField.value = data.application || '';
                prayerField.value = data.prayer || '';

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
                observationField.value = '';
                applicationField.value = '';
                prayerField.value = '';
            });
    }

    function saveData(immediate = false) {
        // Guard against saving with empty date
        if (!currentDate) {
            return;
        }

        // Capture state at the moment of calling
        const dataToSave = {
            date: currentDate, // Use the currentDate scope variable
            observation: observationField.value,
            application: applicationField.value,
            prayer: prayerField.value,
            selectedVerses: selectedVerseIds
        };

        if (immediate) {
            if (saveTimeout) clearTimeout(saveTimeout);
        }

        saveStatus.textContent = 'Saving...';
        saveStatus.className = 'save-status saving';

        fetch('/soap', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify(dataToSave)
        })
            .then(response => response.json())
            .then(result => {
                if (result.error) {
                    saveStatus.textContent = 'Error saving';
                    saveStatus.className = 'save-status error';
                } else {
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
            })
            .catch(error => {
                saveStatus.textContent = 'Error saving';
                saveStatus.className = 'save-status error';
                console.error('Error:', error);
            });
    }

    function scheduleSave() {
        if (saveTimeout) {
            clearTimeout(saveTimeout);
        }
        saveTimeout = setTimeout(saveData, SAVE_DELAY);
    }

    observationField.addEventListener('input', scheduleSave);
    applicationField.addEventListener('input', scheduleSave);
    prayerField.addEventListener('input', scheduleSave);
})();
