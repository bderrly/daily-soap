(function () {
    // Get data from the page (set by inline script in HTML)
    const date = window.SOAP_DATA?.date || '';
    const observationField = document.getElementById('observation');
    const applicationField = document.getElementById('application');
    const prayerField = document.getElementById('prayer');
    const saveStatus = document.getElementById('saveStatus');
    const selectedVersesReference = document.getElementById('selectedVersesReference');

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
    function parseVerseId(verseId) {
        const match = verseId.match(/^v(\d{2})(\d{3})(\d{3})/);
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
        // Check if element itself has verse ID
        if (element.id && element.id.startsWith('v')) {
            const info = parseVerseId(element.id);
            if (info) return info;
        }

        // Traverse up the DOM tree to find verse ID in parent elements
        let current = element;
        while (current && current !== document.body) {
            if (current.id && current.id.startsWith('v')) {
                const info = parseVerseId(current.id);
                if (info) return info;
            }
            current = current.parentElement;
        }

        // If we're inside a verse-content, find the closest preceding verse number
        const verseContent = element.closest('.verse-content');
        if (verseContent) {
            // Get all verse number elements in this container
            const allVerseNums = Array.from(verseContent.querySelectorAll('[id^="v"]'));

            if (allVerseNums.length > 0) {
                // Find the verse number that comes before this element in document order
                // and is closest to it (comes latest before the element)
                let bestVerseNum = null;

                for (const verseNum of allVerseNums) {
                    const position = element.compareDocumentPosition(verseNum);
                    // Check if verseNum comes before the element (or the element is a descendant)
                    if (position & Node.DOCUMENT_POSITION_PRECEDING ||
                        position & Node.DOCUMENT_POSITION_CONTAINS) {
                        // This verse number could be the one
                        if (!bestVerseNum) {
                            bestVerseNum = verseNum;
                        } else {
                            // Check if this verseNum comes after bestVerseNum
                            // If so, it's closer to the clicked element
                            const bestPos = bestVerseNum.compareDocumentPosition(verseNum);
                            if (bestPos & Node.DOCUMENT_POSITION_FOLLOWING) {
                                bestVerseNum = verseNum;
                            }
                        }
                    }
                }

                if (bestVerseNum && bestVerseNum.id) {
                    return parseVerseId(bestVerseNum.id);
                }

                // Fallback: use the first verse number if nothing found
                const firstVerseNum = allVerseNums[0];
                if (firstVerseNum && firstVerseNum.id) {
                    return parseVerseId(firstVerseNum.id);
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

    // Get the base verse ID (without suffix like "-1")
    function getBaseVerseId(verseId) {
        const match = verseId.match(/^(v\d{8})/);
        return match ? match[1] : verseId;
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
            // Select - use the actual verse ID we found
            selectedVerseIds.push(verseInfo.id);
            highlightVerse(verseInfo.id);
        }

        updateVerseReference();
        scheduleSave();
    }

    // Highlight a verse
    function highlightVerse(verseId) {
        const baseId = getBaseVerseId(verseId);
        // Find all elements with IDs starting with the base verse ID
        const allElements = document.querySelectorAll('[id^="' + baseId + '"]');
        allElements.forEach(el => {
            el.classList.add('verse-selected');
        });
    }

    // Remove verse highlight
    function removeVerseHighlight(verseId) {
        const baseId = getBaseVerseId(verseId);
        // Find all elements with IDs starting with the base verse ID
        const allElements = document.querySelectorAll('[id^="' + baseId + '"]');
        allElements.forEach(el => {
            el.classList.remove('verse-selected');
        });
    }

    // Initialize verse selection on page load
    function initializeVerseSelection() {
        // Add click handlers to verse content area
        const versesSection = document.querySelector('.verses-section');
        if (versesSection) {
            versesSection.addEventListener('click', function (e) {
                const verseInfo = getVerseInfo(e.target);
                if (verseInfo) {
                    e.preventDefault();
                    toggleVerseSelection(verseInfo);
                }
            });
        }

        // Highlight initially selected verses (deduplicate by base ID)
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

    // Run initialization when DOM is ready
    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', initializeVerseSelection);
    } else {
        initializeVerseSelection();
    }

    function saveData() {
        const data = {
            date: date,
            observation: observationField.value,
            application: applicationField.value,
            prayer: prayerField.value,
            selectedVerses: selectedVerseIds
        };

        saveStatus.textContent = 'Saving...';
        saveStatus.className = 'save-status saving';

        fetch('/api/soap', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify(data)
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
                        saveStatus.textContent = '';
                        saveStatus.className = 'save-status';
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

