// Authentication Module
// Handles user login, signup, and session management

// Supabase client is initialized in config.js
// Access via window.supabase

// Check if user is already logged in
async function checkAuth() {
  const { data: { session } } = await supabase.auth.getSession();
  
  const isLoginPage = window.location.pathname.endsWith('login.html');
  const isIndexPage = window.location.pathname.endsWith('index.html') || 
                      window.location.pathname.endsWith('/') ||
                      window.location.pathname.endsWith('/Remote/');
  const isAdminPage = window.location.pathname.endsWith('admin.html');
  
  // Index page handles routing, skip auth check there
  if (isIndexPage) {
    return;
  }
  
  if (session && isLoginPage) {
    // Check for redirect target
    const redirectTarget = sessionStorage.getItem('loginRedirect');
    sessionStorage.removeItem('loginRedirect');
    
    if (redirectTarget === 'agent') {
      window.location.href = 'agent.html';
    } else {
      // Let index.html handle role-based routing
      window.location.href = 'index.html';
    }
  } else if (!session && !isLoginPage) {
    // User is not logged in -> redirect to login
    window.location.href = 'login.html';
  } else if (session && !isLoginPage && !isAdminPage) {
    // Check if user is approved (only for dashboard, not admin page)
    const { data: approval, error } = await supabase
      .from('user_approvals')
      .select('approved')
      .eq('user_id', session.user.id)
      .single();
    
    if (error) {
      console.error('Error checking approval status:', error);
    } else if (approval && !approval.approved) {
      // User is not approved - redirect to login with pending status
      await supabase.auth.signOut();
      window.location.href = 'login.html?status=pending';
      return null;
    }
  }
  
  return session;
}

// Login Page Logic
if (document.getElementById('loginForm')) {
  const loginForm = document.getElementById('loginForm');
  const signupForm = document.getElementById('signupForm');
  const showSignupBtn = document.getElementById('showSignupBtn');
  const backToLoginBtn = document.getElementById('backToLoginBtn');
  const authMessage = document.getElementById('authMessage');

  // Toggle between login and signup
  showSignupBtn.addEventListener('click', () => {
    loginForm.style.display = 'none';
    showSignupBtn.style.display = 'none';
    signupForm.style.display = 'block';
    authMessage.style.display = 'none';
  });

  backToLoginBtn.addEventListener('click', () => {
    signupForm.style.display = 'none';
    loginForm.style.display = 'block';
    showSignupBtn.style.display = 'block';
    authMessage.style.display = 'none';
  });

  // Handle login
  loginForm.addEventListener('submit', async (e) => {
    e.preventDefault();
    const email = document.getElementById('email').value;
    const password = document.getElementById('password').value;
    const loginBtn = document.getElementById('loginBtn');

    loginBtn.disabled = true;
    loginBtn.innerHTML = '<span>Signing in...</span>';
    authMessage.style.display = 'none';

    try {
      const { data, error } = await supabase.auth.signInWithPassword({
        email,
        password
      });

      if (error) throw error;

      // Success - redirect to dashboard
      window.location.href = 'dashboard.html';
    } catch (error) {
      authMessage.className = 'message error';
      authMessage.textContent = error.message;
      authMessage.style.display = 'block';
      loginBtn.disabled = false;
      loginBtn.innerHTML = '<span>Sign In</span>';
    }
  });

  // Handle signup
  signupForm.addEventListener('submit', async (e) => {
    e.preventDefault();
    const email = document.getElementById('signupEmail').value;
    const password = document.getElementById('signupPassword').value;
    const passwordConfirm = document.getElementById('signupPasswordConfirm').value;
    const signupBtn = document.getElementById('signupBtn');

    if (password !== passwordConfirm) {
      authMessage.className = 'message error';
      authMessage.textContent = 'Passwords do not match';
      authMessage.style.display = 'block';
      return;
    }

    if (password.length < 6) {
      authMessage.className = 'message error';
      authMessage.textContent = 'Password must be at least 6 characters';
      authMessage.style.display = 'block';
      return;
    }

    signupBtn.disabled = true;
    signupBtn.innerHTML = '<span>Creating account...</span>';
    authMessage.style.display = 'none';

    try {
      const { data, error } = await supabase.auth.signUp({
        email,
        password,
        options: {
          emailRedirectTo: window.location.origin + '/Remote/dashboard.html'
        }
      });

      if (error) throw error;

      authMessage.className = 'message success';
      authMessage.textContent = 'Account created! Check your email to confirm.';
      authMessage.style.display = 'block';
      signupForm.reset();
      
      setTimeout(() => {
        backToLoginBtn.click();
      }, 3000);
    } catch (error) {
      authMessage.className = 'message error';
      authMessage.textContent = error.message;
      authMessage.style.display = 'block';
    } finally {
      signupBtn.disabled = false;
      signupBtn.innerHTML = '<span>Create Account</span>';
    }
  });

  // Check auth on load
  checkAuth();
}

// Dashboard Page Logic
if (document.getElementById('logoutBtn')) {
  const logoutBtn = document.getElementById('logoutBtn');
  const userEmail = document.getElementById('userEmail');
  const adminLink = document.getElementById('adminLink');

  // Display user email and check admin status
  checkAuth().then(async (session) => {
    if (session && session.user) {
      userEmail.textContent = session.user.email;
      
      // Check if user is admin or super_admin
      const { data: approval } = await supabase
        .from('user_approvals')
        .select('role')
        .eq('user_id', session.user.id)
        .single();
      
      const isAdmin = approval && (approval.role === 'admin' || approval.role === 'super_admin');
      
      // Show admin link if user is admin
      if (adminLink && isAdmin) {
        adminLink.style.display = 'inline-flex';
      }
      
      // Add controller download for admins
      const downloadsGrid = document.getElementById('downloadsGrid');
      const downloadsDescription = document.getElementById('downloadsDescription');
      if (downloadsGrid && isAdmin) {
        const controllerLink = document.createElement('a');
        controllerLink.href = 'https://downloads.hawkeye123.dk/controller.exe';
        controllerLink.className = 'btn btn-secondary';
        controllerLink.style.cssText = 'text-decoration: none; text-align: center;';
        controllerLink.innerHTML = '🎮 Controller';
        downloadsGrid.appendChild(controllerLink);
        
        if (downloadsDescription) {
          downloadsDescription.textContent = 'Windows Agent (anbefalet) • Web Agent (browser) • Controller (kontrol enheder)';
        }
      }
    }
  });

  // Handle logout
  logoutBtn.addEventListener('click', async (e) => {
    e.preventDefault();
    console.log('🚪 Logout button clicked');
    try {
      const { error } = await supabase.auth.signOut();
      if (error) {
        console.error('Logout error:', error);
        showToast('Logout fejlede: ' + error.message, 'error');
      } else {
        console.log('✅ Logged out successfully');
        window.location.href = 'login.html?status=logout';
      }
    } catch (err) {
      console.error('Logout exception:', err);
      // Force redirect anyway
      window.location.href = 'login.html?status=logout';
    }
  });
}

// Export for use in other modules
window.checkAuth = checkAuth;
