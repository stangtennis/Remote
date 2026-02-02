// Global Error Boundary for Remote Desktop Dashboard
// Catches unhandled errors and provides user-friendly fallback UI

(function() {
  'use strict';

  let errorCount = 0;
  const MAX_ERRORS = 3;

  // Error boundary UI template
  const errorBoundaryHTML = `
    <div style="
      position: fixed;
      top: 0;
      left: 0;
      width: 100%;
      height: 100%;
      background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
      display: flex;
      align-items: center;
      justify-content: center;
      z-index: 999999;
      font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
    ">
      <div style="
        background: rgba(255, 255, 255, 0.1);
        backdrop-filter: blur(10px);
        border-radius: 20px;
        padding: 3rem;
        max-width: 500px;
        text-align: center;
        border: 1px solid rgba(255, 255, 255, 0.2);
        box-shadow: 0 8px 32px rgba(0, 0, 0, 0.3);
      ">
        <div style="font-size: 4rem; margin-bottom: 1rem;">⚠️</div>
        <h1 style="color: white; margin: 0 0 1rem 0; font-size: 2rem;">Noget gik galt</h1>
        <p style="color: rgba(255, 255, 255, 0.9); margin: 0 0 2rem 0; line-height: 1.6;">
          Der opstod en uventet fejl. Prøv at genindlæse siden.
        </p>
        <div style="display: flex; gap: 1rem; justify-content: center; flex-wrap: wrap;">
          <button onclick="location.reload()" style="
            background: white;
            color: #667eea;
            border: none;
            padding: 12px 24px;
            border-radius: 8px;
            font-size: 1rem;
            font-weight: 600;
            cursor: pointer;
            transition: transform 0.2s;
          " onmouseover="this.style.transform='scale(1.05)'" onmouseout="this.style.transform='scale(1)'">
            🔄 Genindlæs siden
          </button>
          <button onclick="window.location.href='/Remote/'" style="
            background: rgba(255, 255, 255, 0.2);
            color: white;
            border: 1px solid rgba(255, 255, 255, 0.3);
            padding: 12px 24px;
            border-radius: 8px;
            font-size: 1rem;
            font-weight: 600;
            cursor: pointer;
            transition: transform 0.2s;
          " onmouseover="this.style.transform='scale(1.05)'" onmouseout="this.style.transform='scale(1)'">
            🏠 Gå til forsiden
          </button>
        </div>
        <details style="margin-top: 2rem; text-align: left;">
          <summary style="color: rgba(255, 255, 255, 0.7); cursor: pointer; font-size: 0.875rem;">
            Tekniske detaljer
          </summary>
          <pre id="errorDetails" style="
            background: rgba(0, 0, 0, 0.3);
            color: rgba(255, 255, 255, 0.8);
            padding: 1rem;
            border-radius: 8px;
            margin-top: 1rem;
            overflow-x: auto;
            font-size: 0.75rem;
            text-align: left;
          "></pre>
        </details>
      </div>
    </div>
  `;

  // Show error boundary UI
  function showErrorBoundary(error, errorInfo) {
    // Create error boundary container
    const errorContainer = document.createElement('div');
    errorContainer.id = 'error-boundary';
    errorContainer.innerHTML = errorBoundaryHTML;
    
    // Add error details
    const errorDetails = errorContainer.querySelector('#errorDetails');
    if (errorDetails && error) {
      errorDetails.textContent = `${error.message || 'Unknown error'}\n\n${error.stack || 'No stack trace available'}\n\n${errorInfo || ''}`;
    }
    
    // Append to body
    document.body.appendChild(errorContainer);
    
    // Log to console for debugging
    console.error('Error Boundary Triggered:', error, errorInfo);
  }

  // Global error handler
  window.addEventListener('error', function(event) {
    errorCount++;
    
    // Prevent default error handling
    event.preventDefault();
    
    // Show error boundary after multiple errors or critical errors
    if (errorCount >= MAX_ERRORS || event.error?.critical) {
      showErrorBoundary(event.error, `File: ${event.filename}\nLine: ${event.lineno}:${event.colno}`);
    } else {
      // Log error but don't show UI yet
      console.error('Error caught by boundary:', event.error);
    }
    
    return true;
  });

  // Unhandled promise rejection handler
  window.addEventListener('unhandledrejection', function(event) {
    errorCount++;
    
    // Prevent default handling
    event.preventDefault();
    
    // Show error boundary after multiple errors
    if (errorCount >= MAX_ERRORS) {
      showErrorBoundary(
        new Error(event.reason?.message || 'Unhandled Promise Rejection'),
        `Promise: ${event.reason?.stack || JSON.stringify(event.reason)}`
      );
    } else {
      // Log error but don't show UI yet
      console.error('Unhandled promise rejection:', event.reason);
    }
    
    return true;
  });

  // Reset error count after successful operation
  window.addEventListener('load', function() {
    setTimeout(function() {
      if (errorCount < MAX_ERRORS) {
        errorCount = 0;
      }
    }, 5000);
  });

  // Expose function to manually trigger error boundary
  window.showErrorBoundary = showErrorBoundary;

  console.log('✅ Error boundary initialized');
})();
