// Dedicated AI-support client page. Enrollment and client history stay out of
// the main remote-desktop dashboard while using the same safe shared helpers.

document.addEventListener('DOMContentLoaded', async () => {
  const section = document.getElementById('aiSupportClientsSection');
  const accessError = document.getElementById('aiSupportAccessError');
  const session = await checkAuth();
  if (!session) return;

  const roleInfo = await fetchDashboardRole();
  if (!roleInfo.isAdmin) {
    if (section) section.style.display = 'none';
    if (accessError) {
      accessError.textContent = 'Kun admin/super_admin har adgang til AI-support klienter.';
      accessError.style.display = 'block';
    }
    return;
  }

  if (section) {
    section.classList.add('is-visible');
    section.style.display = 'block';
  }
  await loadAISupportClients();
});
