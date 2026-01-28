const { BlogSearch } = require('./search');

describe('BlogSearch', () => {
    let blogSearch;
    const mockData = {
        posts: [
            { id: 'post-1', title: 'Go Programming', date: '2024-01-01', tags: ['go', 'backend'], slug: 'post-1' },
            { id: 'post-2', title: 'JavaScript Guide', date: '2024-01-02', tags: ['js', 'frontend'], slug: 'post-2' }
        ],
        invertedIndex: {
            'go': ['post-1'],
            'programming': ['post-1'],
            'javascript': ['post-2'],
            'guide': ['post-2']
        }
    };

    beforeEach(() => {
        blogSearch = new BlogSearch();
        blogSearch.posts = mockData.posts;
        blogSearch.invertedIndex = mockData.invertedIndex;
        blogSearch.initialized = true;
    });

    test('tokenize should split text and lowercase', () => {
        expect(blogSearch.tokenize('Hello World!')).toEqual(['hello', 'world']);
    });

    test('search should return exact tag matches first', () => {
        const results = blogSearch.search('go');
        expect(results).toHaveLength(1);
        expect(results[0].id).toBe('post-1');
    });

    test('search should return multiple results for common words', () => {
        // Add a common word to mock data
        blogSearch.invertedIndex['code'] = ['post-1', 'post-2'];
        const results = blogSearch.search('code');
        expect(results).toHaveLength(2);
    });

    test('search should be case-insensitive', () => {
        const results = blogSearch.search('JAVASCRIPT');
        expect(results).toHaveLength(1);
        expect(results[0].id).toBe('post-2');
    });

    test('getSuggestions should return matching tags and titles', () => {
        const suggestions = blogSearch.getSuggestions('go');
        expect(suggestions).toContain('go');
        expect(suggestions).toContain('Go Programming');
    });

    test('sortByDate should sort newest first', () => {
        const sorted = blogSearch.sortByDate(mockData.posts);
        expect(sorted[0].id).toBe('post-2');
        expect(sorted[1].id).toBe('post-1');
    });

    test('search should return empty array for no matches', () => {
        const results = blogSearch.search('nonexistent');
        expect(results).toEqual([]);
    });

    test('search should return empty array for empty query', () => {
        expect(blogSearch.search('')).toEqual([]);
        expect(blogSearch.search('   ')).toEqual([]);
    });
});
