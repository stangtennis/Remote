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

  const resetForm = document.getElementById('resetForm');
  const forgotPasswordLink = document.getElementById('forgotPasswordLink');
  const backToLoginFromReset = document.getElementById('backToLoginFromReset');
  const loginDivider = document.getElementById('loginDivider');

  function showLoginView() {
    loginForm.style.display = 'block';
    signupForm.style.display = 'none';
    if (resetForm) resetForm.style.display = 'none';
    showSignupBtn.style.display = 'block';
    if (forgotPasswordLink) forgotPasswordLink.parentElement.style.display = '';
    if (loginDivider) loginDivider.style.display = '';
    authMessage.style.display = 'none';
  }

  // Toggle between login and signup
  showSignupBtn.addEventListener('click', () => {
    loginForm.style.display = 'none';
    showSignupBtn.style.display = 'none';
    if (forgotPasswordLink) forgotPasswordLink.parentElement.style.display = 'none';
    if (loginDivider) loginDivider.style.display = 'none';
    signupForm.style.display = 'block';
    authMessage.style.display = 'none';
  });

  backToLoginBtn.addEventListener('click', showLoginView);

  // Forgot password
  if (forgotPasswordLink) {
    forgotPasswordLink.addEventListener('click', (e) => {
      e.preventDefault();
      loginForm.style.display = 'none';
      showSignupBtn.style.display = 'none';
      if (forgotPasswordLink) forgotPasswordLink.parentElement.style.display = 'none';
      if (loginDivider) loginDivider.style.display = 'none';
      if (resetForm) resetForm.style.display = 'block';
      authMessage.style.display = 'none';
      // Pre-fill email if already entered
      const emailVal = document.getElementById('email')?.value;
      if (emailVal) document.getElementById('resetEmail').value = emailVal;
    });
  }

  if (backToLoginFromReset) {
    backToLoginFromReset.addEventListener('click', showLoginView);
  }

  // Handle password reset
  if (resetForm) {
    resetForm.addEventListener('submit', async (e) => {
      e.preventDefault();
      const email = document.getElementById('resetEmail').value;
      const resetBtn = document.getElementById('resetBtn');

      resetBtn.disabled = true;
      resetBtn.innerHTML = '<span>Sender...</span>';
      authMessage.style.display = 'none';

      try {
        const { error } = await supabase.auth.resetPasswordForEmail(email, {
          redirectTo: window.location.origin + '/Remote/reset-password.html'
        });

        if (error) throw error;

        authMessage.className = 'message success';
        authMessage.textContent = 'Nulstillingslink sendt! Tjek din indbakke (og spam-mappe).';
        authMessage.style.display = 'block';
      } catch (error) {
        authMessage.className = 'message error';
        authMessage.textContent = translateAuthError(error.message);
        authMessage.style.display = 'block';
      } finally {
        resetBtn.disabled = false;
        resetBtn.innerHTML = '<span>Send nulstillingslink</span>';
      }
    });
  }

  // Handle login
  loginForm.addEventListener('submit', async (e) => {
    e.preventDefault();
    const email = document.getElementById('email').value;
    const password = document.getElementById('password').value;
    const loginBtn = document.getElementById('loginBtn');

    loginBtn.disabled = true;
    loginBtn.innerHTML = '<span>Logger ind...</span>';
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
      authMessage.textContent = translateAuthError(error.message);
      authMessage.style.display = 'block';
      loginBtn.disabled = false;
      loginBtn.innerHTML = '<span>Log ind</span>';
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
      authMessage.textContent = 'Adgangskoderne stemmer ikke overens';
      authMessage.style.display = 'block';
      return;
    }

    if (password.length < 6) {
      authMessage.className = 'message error';
      authMessage.textContent = 'Adgangskoden skal være mindst 6 tegn';
      authMessage.style.display = 'block';
      return;
    }

    signupBtn.disabled = true;
    signupBtn.innerHTML = '<span>Opretter konto...</span>';
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
      authMessage.textContent = 'Konto oprettet! Tjek din email for at bekræfte.';
      authMessage.style.display = 'block';
      signupForm.reset();

      setTimeout(() => {
        backToLoginBtn.click();
      }, 3000);
    } catch (error) {
      authMessage.className = 'message error';
      authMessage.textContent = translateAuthError(error.message);
      authMessage.style.display = 'block';
    } finally {
      signupBtn.disabled = false;
      signupBtn.innerHTML = '<span>Opret konto</span>';
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
      
      // Show admin link and Quick Support button if user is admin
      if (adminLink && isAdmin) {
        adminLink.style.display = 'inline-flex';
      }

      const quickSupportBtn = document.getElementById('quickSupportBtn');
      if (quickSupportBtn && isAdmin) {
        quickSupportBtn.style.display = 'inline-flex';
      }
      
      // Add controller download for admins (platform-aware)
      const downloadsGrid = document.getElementById('downloadsGrid');
      const downloadsDescription = document.getElementById('downloadsDescription');
      if (downloadsGrid && isAdmin) {
        const isMac = window._isMacPlatform || /Mac/i.test(navigator.platform);
        const controllerFile = isMac ? 'controller-macos' : 'controller.exe';
        const controllerLabel = isMac ? '🎮 Controller (macOS)' : '🎮 Admin Controller';
        const controllerLink = document.createElement('a');
        controllerLink.href = '#';
        controllerLink.onclick = function() { signedDownload(controllerFile); return false; };
        controllerLink.className = 'btn btn-secondary';
        controllerLink.style.cssText = 'text-decoration: none; text-align: center;';
        controllerLink.innerHTML = controllerLabel;
        downloadsGrid.appendChild(controllerLink);

        if (downloadsDescription) {
          downloadsDescription.textContent = 'Client Agent: installér på den PC der skal fjernstyres • Web Agent: browser-baseret alternativ • Admin Controller: styr enheder fra din PC';
        }
      }
    }
  });

  // Handle logout
  logoutBtn.addEventListener('click', async (e) => {
    e.preventDefault();
    debug('🚪 Logout button clicked');
    try {
      // End all active sessions BEFORE signing out
      if (window.SessionManager && window.SessionManager.getSessionCount() > 0) {
        debug('🔌 Ending all active sessions before logout...');
        const deviceIds = [...window.SessionManager.sessions.keys()];
        for (const deviceId of deviceIds) {
          await window.endSession(deviceId);
        }
        debug('✅ All sessions ended');
      }

      const { error } = await supabase.auth.signOut();
      if (error) {
        console.error('Logout error:', error);
        showToast('Logout fejlede: ' + error.message, 'error');
      } else {
        debug('✅ Logged out successfully');
        window.location.href = 'login.html?status=logout';
      }
    } catch (err) {
      console.error('Logout exception:', err);
      // Force redirect anyway
      window.location.href = 'login.html?status=logout';
    }
  });
}

// Translate Supabase auth error messages to Danish
function translateAuthError(msg) {
  const translations = {
    'Invalid login credentials': 'Forkert email eller adgangskode',
    'Email not confirmed': 'Email er ikke bekræftet — tjek din indbakke',
    'User already registered': 'Brugeren er allerede registreret',
    'Password should be at least 6 characters': 'Adgangskoden skal være mindst 6 tegn',
    'Signup requires a valid password': 'Tilmelding kræver en gyldig adgangskode',
    'Unable to validate email address: invalid format': 'Ugyldig email-adresse',
    'Email rate limit exceeded': 'For mange forsøg — prøv igen senere',
    'For security purposes, you can only request this after': 'Af sikkerhedshensyn kan du kun anmode om dette efter',
    'Too many requests': 'For mange forsøg — vent venligst',
    'Network error': 'Netværksfejl — tjek din forbindelse',
  };

  for (const [en, da] of Object.entries(translations)) {
    if (msg.includes(en)) return da;
  }
  return msg;
}

// Export for use in other modules
window.checkAuth = checkAuth;
