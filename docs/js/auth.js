// Authentication Module
// Handles user login, signup, and session management

const SUPABASE_URL = 'https://supabase.hawkeye123.dk';
const SUPABASE_ANON_KEY = 'eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyAgCiAgICAicm9sZSI6ICJhbm9uIiwKICAgICJpc3MiOiAic3VwYWJhc2UtZGVtbyIsCiAgICAiaWF0IjogMTY0MTc2OTIwMCwKICAgICJleHAiOiAxNzk5NTM1NjAwCn0.dc_X5iR_VP_qT0zsiyj_I_OZ2T9FtRU2BBNWN8Bu4GE';

// Initialize Supabase client
const supabase = window.supabase.createClient(SUPABASE_URL, SUPABASE_ANON_KEY);

// Check if user is already logged in
async function checkAuth() {
  const { data: { session } } = await supabase.auth.getSession();
  
  const isLoginPage = window.location.pathname.endsWith('index.html') || 
                      window.location.pathname.endsWith('/') ||
                      window.location.pathname.endsWith('/Remote/');
  const isAdminPage = window.location.pathname.endsWith('admin.html');
  
  if (session && isLoginPage) {
    // User is logged in on login page -> redirect to dashboard
    window.location.href = 'dashboard.html';
  } else if (!session && !isLoginPage) {
    // User is not logged in on dashboard -> redirect to login
    window.location.href = 'index.html';
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
      // User is not approved - show message and logout
      alert('⏸️ Your account is pending approval.\n\nPlease wait for an administrator to approve your account before you can access the dashboard.');
      await supabase.auth.signOut();
      window.location.href = 'index.html';
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

  // Display user email
  checkAuth().then(session => {
    if (session && session.user) {
      userEmail.textContent = session.user.email;
    }
  });

  // Handle logout
  logoutBtn.addEventListener('click', async () => {
    const { error } = await supabase.auth.signOut();
    if (!error) {
      window.location.href = 'index.html';
    }
  });
}

// Export for use in other modules
window.supabase = supabase;
window.checkAuth = checkAuth;
