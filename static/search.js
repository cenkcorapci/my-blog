/**
 * Client-Side Search Module for Static Site Deployment
 * 
 * This script provides full-text search and tag filtering functionality
 * for the static version of the blog.
 * 
 * It loads a pre-generated search index (search-index.json) and performs
 * all search operations in the browser.
 */

class BlogSearch {
    constructor() {
        this.posts = [];
        this.invertedIndex = {};
        this.initialized = false;
    }

    /**
     * Initialize the search engine by loading the search index
     */
    async init() {
        if (this.initialized) return;

        try {
            // Fetch relative to root to ensure it works on subpages
            const response = await fetch('/search-index.json');
            if (!response.ok) {
                throw new Error('Failed to load search index');
            }

            const data = await response.json();
            // Ensure posts and tags are arrays to prevent iteration errors
            this.posts = (data.posts || []).map(post => ({
                ...post,
                tags: post.tags || []
            }));
            this.invertedIndex = data.invertedIndex || {};
            this.initialized = true;
            console.log('Search index loaded successfully');
        } catch (error) {
            console.error('Error loading search index:', error);
            this.posts = [];
            this.invertedIndex = {};
            this.initialized = true;
        }
    }

    /**
     * Tokenize text into searchable words
     */
    tokenize(text) {
        if (!text) return [];
        return (text.match(/[a-zA-Z0-9]+/g) || []).map(w => w.toLowerCase());
    }

    /**
     * Search for posts matching a query
     * @param {string} query - The search query
     * @returns {Array} Array of matching posts
     */
    search(query) {
        if (!this.initialized || !query) return [];

        query = query.trim().toLowerCase();
        if (!query) return [];

        // First, check for exact tag match
        const tagMatches = this.posts.filter(post =>
            (post.tags || []).some(tag => tag.toLowerCase() === query)
        );

        if (tagMatches.length > 0) {
            return this.sortByDate(tagMatches);
        }

        // Tokenize search query and find posts matching all words (AND search)
        const words = this.tokenize(query);
        if (words.length === 0) return [];

        let matchingPostIds = null;

        for (const word of words) {
            const postIds = this.invertedIndex[word] || [];

            if (matchingPostIds === null) {
                matchingPostIds = new Set(postIds);
            } else {
                // Intersection: found both in current set AND word index
                const intersection = new Set();
                for (const id of postIds) {
                    if (matchingPostIds.has(id)) {
                        intersection.add(id);
                    }
                }
                matchingPostIds = intersection;
            }

            if (matchingPostIds.size === 0) break;
        }

        if (!matchingPostIds || matchingPostIds.size === 0) return [];

        const results = this.posts.filter(post => matchingPostIds.has(post.id));
        return this.sortByDate(results);
    }

    /**
     * Get search suggestions based on partial query
     * @param {string} query - The partial search query
     * @returns {Array} Array of suggestion strings
     */
    getSuggestions(query) {
        if (!this.initialized || !query) return [];

        query = query.trim().toLowerCase();
        if (query.length < 2) return [];

        const suggestions = new Set();

        for (const post of this.posts) {
            // Suggest from tags
            const tags = post.tags || [];
            for (const tag of tags) {
                if (tag.toLowerCase().startsWith(query)) {
                    suggestions.add(tag);
                }
            }

            // Suggest from titles
            if (post.title.toLowerCase().includes(query)) {
                suggestions.add(post.title);
            }
        }

        return Array.from(suggestions).slice(0, 10);
    }

    /**
     * Sort posts by date (newest first)
     */
    sortByDate(posts) {
        return [...posts].sort((a, b) => new Date(b.date) - new Date(a.date));
    }
}

// Global search instance
const blogSearch = new BlogSearch();

/**
 * Render posts to the page
 */
function renderPosts(posts, containerId) {
    const container = document.getElementById(containerId);
    if (!container) return;

    if (!posts || posts.length === 0) {
        container.innerHTML = '<p class="no-results">No posts found matching your search.</p>';
        return;
    }

    const html = posts.map(post => {
        const date = new Date(post.date);
        const formattedDate = date.toLocaleDateString('en-US', {
            year: 'numeric',
            month: 'long',
            day: 'numeric'
        });
        const isoDate = post.date;

        const tags = post.tags || [];
        const tagsHtml = tags.length > 0 ? `
            <div class="post-tags">
                ${tags.map(tag => `<a href="/search/?q=${encodeURIComponent(tag)}" class="tag">${tag}</a>`).join('')}
            </div>
        ` : '';

        return `
            <article class="post-card">
                <time datetime="${isoDate}">${formattedDate}</time>
                <h3><a href="/post/${post.slug}/">${post.title}</a></h3>
                ${tagsHtml}
            </article>
        `;
    }).join('');

    container.innerHTML = html;
}

/**
 * Initialize client-side search functionality
 */
async function initClientSearch() {
    await blogSearch.init();

    const searchInput = document.getElementById('search-input');
    const suggestionsList = document.getElementById('suggestions');
    const searchResultsContainer = document.getElementById('search-results');

    if (!searchInput || !suggestionsList) return;

    // Handle search on page load (for /search/?q=query URLs)
    const urlParams = new URLSearchParams(window.location.search);
    const initialQuery = urlParams.get('q');

    if (initialQuery && searchResultsContainer) {
        searchInput.value = initialQuery;
        const results = blogSearch.search(initialQuery);
        renderPosts(results, 'search-results');

        // Update the heading if it exists
        const searchTitle = document.getElementById('search-title');
        if (searchTitle) {
            searchTitle.textContent = `Search Results for "${initialQuery}"`;
        }
    }

    let selectedSuggestionIndex = -1;

    // Handle suggestions on input
    searchInput.addEventListener('input', (e) => {
        const query = e.target.value.trim();
        selectedSuggestionIndex = -1; // Reset selection

        if (query.length < 2) {
            suggestionsList.style.display = 'none';
            return;
        }

        const suggestions = blogSearch.getSuggestions(query);

        if (suggestions.length > 0) {
            suggestionsList.innerHTML = suggestions
                .map((s, index) => `<div class="suggestion-item" data-index="${index}">${s}</div>`)
                .join('');
            suggestionsList.style.display = 'block';

            suggestionsList.querySelectorAll('.suggestion-item').forEach(item => {
                item.addEventListener('click', () => {
                    searchInput.value = item.textContent;
                    suggestionsList.style.display = 'none';
                    window.location.href = `/search/?q=${encodeURIComponent(item.textContent)}`;
                });
            });
        } else {
            suggestionsList.style.display = 'none';
        }
    });

    // Handle keyboard navigation
    searchInput.addEventListener('keydown', (e) => {
        const items = suggestionsList.querySelectorAll('.suggestion-item');
        if (items.length === 0) return;

        if (e.key === 'ArrowDown') {
            e.preventDefault();
            selectedSuggestionIndex = (selectedSuggestionIndex + 1) % items.length;
            updateSelectedSuggestion(items);
        } else if (e.key === 'ArrowUp') {
            e.preventDefault();
            selectedSuggestionIndex = (selectedSuggestionIndex - 1 + items.length) % items.length;
            updateSelectedSuggestion(items);
        } else if (e.key === 'Enter' && selectedSuggestionIndex >= 0) {
            e.preventDefault();
            const selectedItem = items[selectedSuggestionIndex];
            searchInput.value = selectedItem.textContent;
            suggestionsList.style.display = 'none';
            window.location.href = `/search/?q=${encodeURIComponent(selectedItem.textContent)}`;
        } else if (e.key === 'Escape') {
            suggestionsList.style.display = 'none';
        }
    });

    function updateSelectedSuggestion(items) {
        items.forEach((item, index) => {
            if (index === selectedSuggestionIndex) {
                item.classList.add('selected');
                item.scrollIntoView({ block: 'nearest' });
            } else {
                item.classList.remove('selected');
            }
        });
    }

    // Handle form submission
    const searchForm = searchInput.closest('form');
    if (searchForm) {
        searchForm.addEventListener('submit', (e) => {
            // Only submit if no suggestion is currently highlighted by keyboard
            if (selectedSuggestionIndex >= 0) return;

            e.preventDefault();
            const query = searchInput.value.trim();
            if (query) {
                window.location.href = `/search/?q=${encodeURIComponent(query)}`;
            }
        });
    }

    // Close suggestions on outside click
    document.addEventListener('click', (e) => {
        if (!searchInput.contains(e.target) && !suggestionsList.contains(e.target)) {
            suggestionsList.style.display = 'none';
        }
    });
}

// Export for use in templates and testing
if (typeof window !== 'undefined') {
    window.BlogSearch = BlogSearch;
    window.blogSearch = blogSearch;
    window.initClientSearch = initClientSearch;
    window.renderPosts = renderPosts;
}

if (typeof module !== 'undefined' && module.exports) {
    module.exports = { BlogSearch, blogSearch };
}
