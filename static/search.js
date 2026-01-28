/**
 * Client-Side Search Module for Static Site Deployment
 * 
 * This script provides full-text search and tag filtering functionality
 * for the static version of the blog (deployed to Netlify).
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
            const response = await fetch('/search-index.json');
            if (!response.ok) {
                throw new Error('Failed to load search index');
            }

            const data = await response.json();
            this.posts = data.posts || [];
            this.invertedIndex = data.invertedIndex || {};
            this.initialized = true;
        } catch (error) {
            console.error('Error loading search index:', error);
            // Fallback: search will return empty results
            this.posts = [];
            this.invertedIndex = {};
            this.initialized = true;
        }
    }

    /**
     * Tokenize text into searchable words
     */
    tokenize(text) {
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
            post.tags.some(tag => tag.toLowerCase() === query)
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
                // Intersection
                matchingPostIds = new Set([...matchingPostIds].filter(id => postIds.includes(id)));
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

        // Suggest from tags
        for (const post of this.posts) {
            for (const tag of post.tags) {
                if (tag.toLowerCase().startsWith(query)) {
                    suggestions.add(tag);
                }
            }
        }

        // Suggest from titles
        for (const post of this.posts) {
            if (post.title.toLowerCase().includes(query)) {
                suggestions.add(post.title);
            }
        }

        // Limit to 10 suggestions
        return Array.from(suggestions).slice(0, 10);
    }

    /**
     * Sort posts by date (newest first)
     */
    sortByDate(posts) {
        return [...posts].sort((a, b) => new Date(b.date) - new Date(a.date));
    }

    /**
     * Get all posts
     */
    getAllPosts() {
        return this.sortByDate(this.posts);
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

    if (posts.length === 0) {
        container.innerHTML = '<p>No posts found matching your search.</p>';
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

        const tagsHtml = post.tags.length > 0 ? `
            <div class="post-tags">
                ${post.tags.map(tag => `<a href="/search/?q=${encodeURIComponent(tag)}" class="tag">${tag}</a>`).join('')}
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

    if (!searchInput) return;

    // Handle search on page load (for /search?q=query URLs)
    const urlParams = new URLSearchParams(window.location.search);
    const initialQuery = urlParams.get('q');

    if (initialQuery && searchResultsContainer) {
        searchInput.value = initialQuery;
        const results = blogSearch.search(initialQuery);
        renderPosts(results, 'search-results');

        // Update the heading
        const heading = document.querySelector('main h2');
        if (heading) {
            heading.textContent = `Search Results for "${initialQuery}"`;
        }
    }

    // Handle suggestions on input
    searchInput.addEventListener('input', (e) => {
        const query = e.target.value.trim();

        if (query.length < 2) {
            suggestionsList.style.display = 'none';
            return;
        }

        const suggestions = blogSearch.getSuggestions(query);

        if (suggestions.length > 0) {
            suggestionsList.innerHTML = suggestions
                .map(s => `<div class="suggestion-item">${s}</div>`)
                .join('');
            suggestionsList.style.display = 'block';

            suggestionsList.querySelectorAll('.suggestion-item').forEach(item => {
                item.addEventListener('click', () => {
                    searchInput.value = item.textContent;
                    // Navigate to search page with query
                    window.location.href = `/search/?q=${encodeURIComponent(item.textContent)}`;
                });
            });
        } else {
            suggestionsList.style.display = 'none';
        }
    });

    // Handle form submission
    const searchForm = searchInput.closest('form');
    if (searchForm) {
        searchForm.addEventListener('submit', (e) => {
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

// Export for use in templates
if (typeof window !== 'undefined') {
    window.BlogSearch = BlogSearch;
    window.blogSearch = blogSearch;
    window.initClientSearch = initClientSearch;
    window.renderPosts = renderPosts;
}
