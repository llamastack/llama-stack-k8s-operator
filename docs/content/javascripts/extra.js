// Enhanced functionality for LlamaStack Operator Documentation

document.addEventListener('DOMContentLoaded', function() {
    // Initialize all enhancements
    initializeCodeCopyButtons();
    initializeAPIExplorer();
    initializeSearchEnhancements();
    initializeNavigationEnhancements();
    initializeAccessibility();
    initializeAnalytics();
});

/**
 * Enhanced copy buttons for code blocks
 */
function initializeCodeCopyButtons() {
    // Add copy success feedback
    document.addEventListener('clipboard-success', function(e) {
        const button = e.detail.trigger;
        const originalText = button.textContent;
        
        button.textContent = 'Copied!';
        button.style.background = '#10b981';
        
        setTimeout(() => {
            button.textContent = originalText;
            button.style.background = '';
        }, 2000);
    });

    // Add copy error feedback
    document.addEventListener('clipboard-error', function(e) {
        const button = e.detail.trigger;
        const originalText = button.textContent;
        
        button.textContent = 'Failed';
        button.style.background = '#ef4444';
        
        setTimeout(() => {
            button.textContent = originalText;
            button.style.background = '';
        }, 2000);
    });
}

/**
 * Interactive API Explorer
 */
function initializeAPIExplorer() {
    // YAML validator for LlamaStackDistribution specs
    const yamlInputs = document.querySelectorAll('.yaml-validator');
    
    yamlInputs.forEach(input => {
        const validateButton = document.createElement('button');
        validateButton.textContent = 'Validate YAML';
        validateButton.className = 'md-button md-button--primary yaml-validate-btn';
        
        validateButton.addEventListener('click', function() {
            validateYAML(input);
        });
        
        input.parentNode.insertBefore(validateButton, input.nextSibling);
    });

    // Add interactive examples
    addInteractiveExamples();
}

/**
 * Validate YAML content
 */
function validateYAML(input) {
    const content = input.value;
    const resultDiv = getOrCreateResultDiv(input);
    
    try {
        // Basic YAML validation (you might want to use a proper YAML parser)
        if (!content.trim()) {
            throw new Error('Empty YAML content');
        }
        
        // Check for basic LlamaStackDistribution structure
        if (!content.includes('apiVersion: llamastack.io/v1alpha1')) {
            throw new Error('Missing required apiVersion');
        }
        
        if (!content.includes('kind: LlamaStackDistribution')) {
            throw new Error('Missing required kind');
        }
        
        resultDiv.innerHTML = `
            <div class="validation-success">
                <strong>✅ Valid YAML</strong>
                <p>Your LlamaStackDistribution configuration appears to be valid.</p>
            </div>
        `;
        resultDiv.className = 'validation-result success';
        
    } catch (error) {
        resultDiv.innerHTML = `
            <div class="validation-error">
                <strong>❌ Invalid YAML</strong>
                <p>Error: ${error.message}</p>
            </div>
        `;
        resultDiv.className = 'validation-result error';
    }
}

/**
 * Get or create result div for validation
 */
function getOrCreateResultDiv(input) {
    let resultDiv = input.parentNode.querySelector('.validation-result');
    if (!resultDiv) {
        resultDiv = document.createElement('div');
        resultDiv.className = 'validation-result';
        input.parentNode.appendChild(resultDiv);
    }
    return resultDiv;
}

/**
 * Add interactive examples
 */
function addInteractiveExamples() {
    const examples = document.querySelectorAll('.interactive-example');
    
    examples.forEach(example => {
        const tryButton = document.createElement('button');
        tryButton.textContent = 'Try this example';
        tryButton.className = 'md-button try-example-btn';
        
        tryButton.addEventListener('click', function() {
            const codeBlock = example.querySelector('code');
            if (codeBlock) {
                copyToClipboard(codeBlock.textContent);
                showNotification('Example copied to clipboard!');
            }
        });
        
        example.appendChild(tryButton);
    });
}

/**
 * Enhanced search functionality
 */
function initializeSearchEnhancements() {
    const searchInput = document.querySelector('.md-search__input');
    if (!searchInput) return;

    // Add search suggestions
    const suggestionsDiv = document.createElement('div');
    suggestionsDiv.className = 'search-suggestions';
    suggestionsDiv.style.display = 'none';
    searchInput.parentNode.appendChild(suggestionsDiv);

    // Popular search terms
    const popularSearches = [
        'installation',
        'quick start',
        'API reference',
        'examples',
        'troubleshooting',
        'configuration',
        'scaling',
        'storage'
    ];

    searchInput.addEventListener('focus', function() {
        if (!this.value) {
            showSearchSuggestions(popularSearches, suggestionsDiv);
        }
    });

    searchInput.addEventListener('blur', function() {
        setTimeout(() => {
            suggestionsDiv.style.display = 'none';
        }, 200);
    });

    // Search analytics
    searchInput.addEventListener('input', function() {
        if (this.value.length > 2) {
            trackSearchQuery(this.value);
        }
    });
}

/**
 * Show search suggestions
 */
function showSearchSuggestions(suggestions, container) {
    container.innerHTML = suggestions.map(term => 
        `<div class="search-suggestion" onclick="performSearch('${term}')">${term}</div>`
    ).join('');
    container.style.display = 'block';
}

/**
 * Perform search
 */
function performSearch(term) {
    const searchInput = document.querySelector('.md-search__input');
    if (searchInput) {
        searchInput.value = term;
        searchInput.dispatchEvent(new Event('input'));
        searchInput.focus();
    }
}

/**
 * Navigation enhancements
 */
function initializeNavigationEnhancements() {
    // Add breadcrumb navigation
    addBreadcrumbs();
    
    // Add "Edit this page" links
    addEditLinks();
    
    // Add page navigation (previous/next)
    addPageNavigation();
    
    // Smooth scrolling for anchor links
    document.querySelectorAll('a[href^="#"]').forEach(anchor => {
        anchor.addEventListener('click', function (e) {
            e.preventDefault();
            const target = document.querySelector(this.getAttribute('href'));
            if (target) {
                target.scrollIntoView({
                    behavior: 'smooth',
                    block: 'start'
                });
            }
        });
    });
}

/**
 * Add breadcrumb navigation
 */
function addBreadcrumbs() {
    const nav = document.querySelector('.md-nav--primary');
    if (!nav) return;

    const currentPath = window.location.pathname;
    const pathParts = currentPath.split('/').filter(part => part);
    
    if (pathParts.length > 1) {
        const breadcrumbContainer = document.createElement('nav');
        breadcrumbContainer.className = 'breadcrumb-nav';
        breadcrumbContainer.setAttribute('aria-label', 'Breadcrumb');
        
        let breadcrumbHTML = '<ol class="breadcrumb">';
        breadcrumbHTML += '<li><a href="/">Home</a></li>';
        
        let currentPath = '';
        pathParts.forEach((part, index) => {
            currentPath += '/' + part;
            const isLast = index === pathParts.length - 1;
            const displayName = part.replace(/-/g, ' ').replace(/\b\w/g, l => l.toUpperCase());
            
            if (isLast) {
                breadcrumbHTML += `<li aria-current="page">${displayName}</li>`;
            } else {
                breadcrumbHTML += `<li><a href="${currentPath}/">${displayName}</a></li>`;
            }
        });
        
        breadcrumbHTML += '</ol>';
        breadcrumbContainer.innerHTML = breadcrumbHTML;
        
        const content = document.querySelector('.md-content');
        if (content) {
            content.insertBefore(breadcrumbContainer, content.firstChild);
        }
    }
}

/**
 * Add edit links
 */
function addEditLinks() {
    const repoUrl = 'https://github.com/llamastack/llama-stack-k8s-operator';
    const currentPath = window.location.pathname;
    const editUrl = `${repoUrl}/edit/main/docs/content${currentPath.replace(/\/$/, '')}.md`;
    
    const editLink = document.createElement('a');
    editLink.href = editUrl;
    editLink.textContent = '✏️ Edit this page';
    editLink.className = 'edit-link';
    editLink.target = '_blank';
    editLink.rel = 'noopener noreferrer';
    
    const article = document.querySelector('article');
    if (article) {
        article.appendChild(editLink);
    }
}

/**
 * Add page navigation
 */
function addPageNavigation() {
    // This would require parsing the navigation structure
    // Implementation depends on MkDocs navigation data
}

/**
 * Accessibility enhancements
 */
function initializeAccessibility() {
    // Add skip to content link
    const skipLink = document.createElement('a');
    skipLink.href = '#main-content';
    skipLink.textContent = 'Skip to main content';
    skipLink.className = 'skip-link';
    document.body.insertBefore(skipLink, document.body.firstChild);
    
    // Mark main content
    const mainContent = document.querySelector('.md-content');
    if (mainContent) {
        mainContent.id = 'main-content';
    }
    
    // Enhance keyboard navigation
    document.addEventListener('keydown', function(e) {
        // Alt + S for search
        if (e.altKey && e.key === 's') {
            e.preventDefault();
            const searchInput = document.querySelector('.md-search__input');
            if (searchInput) {
                searchInput.focus();
            }
        }
        
        // Alt + H for home
        if (e.altKey && e.key === 'h') {
            e.preventDefault();
            window.location.href = '/';
        }
    });
    
    // Add ARIA labels to interactive elements
    document.querySelectorAll('.md-nav__link').forEach(link => {
        if (!link.getAttribute('aria-label')) {
            link.setAttribute('aria-label', `Navigate to ${link.textContent.trim()}`);
        }
    });
}

/**
 * Analytics and tracking
 */
function initializeAnalytics() {
    // Track page views
    trackPageView();
    
    // Track user interactions
    trackUserInteractions();
    
    // Track performance metrics
    trackPerformanceMetrics();
}

/**
 * Track page view
 */
function trackPageView() {
    // Implementation depends on your analytics provider
    console.log('Page view tracked:', window.location.pathname);
}

/**
 * Track search queries
 */
function trackSearchQuery(query) {
    // Implementation depends on your analytics provider
    console.log('Search query tracked:', query);
}

/**
 * Track user interactions
 */
function trackUserInteractions() {
    // Track copy button clicks
    document.addEventListener('click', function(e) {
        if (e.target.classList.contains('md-clipboard')) {
            console.log('Copy button clicked');
        }
        
        if (e.target.classList.contains('try-example-btn')) {
            console.log('Try example button clicked');
        }
    });
}

/**
 * Track performance metrics
 */
function trackPerformanceMetrics() {
    // Track page load time
    window.addEventListener('load', function() {
        const loadTime = performance.now();
        console.log('Page load time:', loadTime);
    });
}

/**
 * Utility functions
 */

/**
 * Copy text to clipboard
 */
function copyToClipboard(text) {
    if (navigator.clipboard) {
        navigator.clipboard.writeText(text);
    } else {
        // Fallback for older browsers
        const textArea = document.createElement('textarea');
        textArea.value = text;
        document.body.appendChild(textArea);
        textArea.select();
        document.execCommand('copy');
        document.body.removeChild(textArea);
    }
}

/**
 * Show notification
 */
function showNotification(message, type = 'info') {
    const notification = document.createElement('div');
    notification.className = `notification notification--${type}`;
    notification.textContent = message;
    
    document.body.appendChild(notification);
    
    // Animate in
    setTimeout(() => {
        notification.classList.add('notification--visible');
    }, 100);
    
    // Remove after 3 seconds
    setTimeout(() => {
        notification.classList.remove('notification--visible');
        setTimeout(() => {
            document.body.removeChild(notification);
        }, 300);
    }, 3000);
}

/**
 * Debounce function
 */
function debounce(func, wait) {
    let timeout;
    return function executedFunction(...args) {
        const later = () => {
            clearTimeout(timeout);
            func(...args);
        };
        clearTimeout(timeout);
        timeout = setTimeout(later, wait);
    };
}

/**
 * Throttle function
 */
function throttle(func, limit) {
    let inThrottle;
    return function() {
        const args = arguments;
        const context = this;
        if (!inThrottle) {
            func.apply(context, args);
            inThrottle = true;
            setTimeout(() => inThrottle = false, limit);
        }
    };
}

// Add CSS for notifications and other dynamic elements
const dynamicStyles = `
    .notification {
        position: fixed;
        top: 20px;
        right: 20px;
        padding: 12px 20px;
        border-radius: 6px;
        color: white;
        font-weight: 500;
        z-index: 1000;
        transform: translateX(100%);
        transition: transform 0.3s ease;
    }
    
    .notification--visible {
        transform: translateX(0);
    }
    
    .notification--info {
        background: #2563eb;
    }
    
    .notification--success {
        background: #10b981;
    }
    
    .notification--error {
        background: #ef4444;
    }
    
    .breadcrumb-nav {
        margin-bottom: 2rem;
        padding: 1rem 0;
        border-bottom: 1px solid var(--md-default-fg-color--lightest);
    }
    
    .breadcrumb {
        list-style: none;
        padding: 0;
        margin: 0;
        display: flex;
        align-items: center;
        gap: 0.5rem;
    }
    
    .breadcrumb li:not(:last-child)::after {
        content: "›";
        margin-left: 0.5rem;
        color: var(--md-default-fg-color--light);
    }
    
    .breadcrumb a {
        color: var(--md-default-fg-color--light);
        text-decoration: none;
    }
    
    .breadcrumb a:hover {
        color: var(--md-primary-fg-color);
    }
    
    .edit-link {
        display: inline-block;
        margin-top: 2rem;
        padding: 0.5rem 1rem;
        background: var(--md-default-fg-color--lightest);
        border-radius: 4px;
        text-decoration: none;
        font-size: 0.9em;
    }
    
    .edit-link:hover {
        background: var(--md-default-fg-color--lighter);
    }
    
    .search-suggestions {
        position: absolute;
        top: 100%;
        left: 0;
        right: 0;
        background: var(--md-default-bg-color);
        border: 1px solid var(--md-default-fg-color--lightest);
        border-radius: 4px;
        box-shadow: 0 4px 6px rgba(0, 0, 0, 0.1);
        z-index: 100;
    }
    
    .search-suggestion {
        padding: 0.5rem 1rem;
        cursor: pointer;
        border-bottom: 1px solid var(--md-default-fg-color--lightest);
    }
    
    .search-suggestion:hover {
        background: var(--md-default-fg-color--lightest);
    }
    
    .search-suggestion:last-child {
        border-bottom: none;
    }
    
    .validation-result {
        margin-top: 1rem;
        padding: 1rem;
        border-radius: 4px;
    }
    
    .validation-result.success {
        background: rgba(16, 185, 129, 0.1);
        border: 1px solid #10b981;
    }
    
    .validation-result.error {
        background: rgba(239, 68, 68, 0.1);
        border: 1px solid #ef4444;
    }
    
    .yaml-validate-btn {
        margin-top: 0.5rem;
        margin-bottom: 1rem;
    }
    
    .try-example-btn {
        margin-top: 1rem;
    }
`;

// Inject dynamic styles
const styleSheet = document.createElement('style');
styleSheet.textContent = dynamicStyles;
document.head.appendChild(styleSheet);
