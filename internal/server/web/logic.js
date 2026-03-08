// Book name mapping (ESV Bible order, 1-indexed, so book 1 = Genesis, book 23 = Isaiah)
export const bookNames = {
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

/**
 * Parse verse ID (format: 23063008)
 * @param {string} verseId
 * @returns {{book: number, chapter: number, verse: number, id: string} | null}
 */
export function parseVerseId(verseId) {
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

/**
 * Format selected verses as reference string (e.g., "Isaiah 63:8-9")
 * @param {string[]} verseIds
 * @returns {string}
 */
export function formatVerseReference(verseIds) {
    if (verseIds.length === 0) return '';

    // Group verses by book and chapter
    const groups = {};
    for (const verseId of verseIds) {
        const info = parseVerseId(verseId);
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
