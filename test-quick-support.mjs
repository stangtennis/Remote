// Playwright test: Quick Support → Main Preview Integration
// Creates a support session, fakes screen sharing via Playwright,
// so the dashboard shows video in main preview.

import { chromium } from 'playwright';
import { createClient } from '@supabase/supabase-js';

const SUPABASE_URL = 'https://supabase.hawkeye123.dk';
const SERVICE_ROLE_KEY = 'eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyAgCiAgICAicm9sZSI6ICJzZXJ2aWNlX3JvbGUiLAogICAgImlzcyI6ICJzdXBhYmFzZS1kZW1vIiwKICAgICJpYXQiOiAxNjQxNzY5MjAwLAogICAgImV4cCI6IDE3OTk1MzU2MDAKfQ.DaYlNEoUrrEn2Ig7tqibS-PHK5vgusbcbo7X36XVt4Q';
const ANON_KEY = 'eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyAgCiAgICAicm9sZSI6ICJhbm9uIiwKICAgICJpc3MiOiAic3VwYWJhc2UtZGVtbyIsCiAgICAiaWF0IjogMTY0MTc2OTIwMCwKICAgICJleHAiOiAxNzk5NTM1NjAwCn0.dc_X5iR_VP_qT0zsiyj_I_OZ2T9FtRU2BBNWN8Bu4GE';
const SITE_URL = 'https://stangtennis.github.io/Remote';

async function main() {
  // 1. Create support session directly via service role
  const supabase = createClient(SUPABASE_URL, SERVICE_ROLE_KEY);

  // Get an admin user for created_by
  const { data: adminUser } = await supabase
    .from('user_approvals')
    .select('user_id')
    .in('role', ['admin', 'super_admin'])
    .limit(1)
    .single();

  if (!adminUser) {
    console.error('No admin user found');
    process.exit(1);
  }

  const pin = Math.floor(100000 + Math.random() * 900000).toString();
  const token = crypto.randomUUID();
  const expires_at = new Date(Date.now() + 30 * 60 * 1000).toISOString();

  const { data: session, error: sessionError } = await supabase
    .from('support_sessions')
    .insert({
      created_by: adminUser.user_id,
      status: 'pending',
      pin,
      token,
      expires_at,
    })
    .select()
    .single();

  if (sessionError) {
    console.error('Failed to create session:', sessionError);
    process.exit(1);
  }

  const sessionId = session.id;
  const shareUrl = `${SITE_URL}/support.html?token=${token}`;
  const dashboardUrl = `${SITE_URL}/dashboard.html?support=${sessionId}`;

  console.log('');
  console.log('='.repeat(60));
  console.log('  QUICK SUPPORT TEST SESSION');
  console.log('='.repeat(60));
  console.log(`  PIN:        ${pin}`);
  console.log(`  Session ID: ${sessionId}`);
  console.log(`  Share URL:  ${shareUrl}`);
  console.log('');
  console.log('  Åbn dette link på dit dashboard:');
  console.log(`  ${dashboardUrl}`);
  console.log('='.repeat(60));
  console.log('');

  // 2. Launch Playwright browser with fake media
  console.log('Starter Playwright browser...');
  const browser = await chromium.launch({
    headless: true,
    args: [
      '--use-fake-device-for-media-stream',
      '--use-fake-ui-for-media-stream',
      '--auto-select-desktop-capture-source=Entire screen',
      '--enable-features=GetDisplayMediaSet',
    ],
  });

  const context = await browser.newContext({
    permissions: ['camera', 'microphone'],
  });

  const page = await context.newPage();

  // Override getDisplayMedia to return a fake canvas stream
  await page.addInitScript(() => {
    navigator.mediaDevices.getDisplayMedia = async () => {
      const canvas = document.createElement('canvas');
      canvas.width = 1280;
      canvas.height = 720;
      const ctx = canvas.getContext('2d');

      // Animate a fake "screen" with moving elements
      let frame = 0;
      function draw() {
        frame++;
        // Background gradient
        const grad = ctx.createLinearGradient(0, 0, 1280, 720);
        grad.addColorStop(0, '#1a1a2e');
        grad.addColorStop(1, '#16213e');
        ctx.fillStyle = grad;
        ctx.fillRect(0, 0, 1280, 720);

        // Moving circle
        const x = 640 + Math.sin(frame * 0.02) * 300;
        const y = 360 + Math.cos(frame * 0.03) * 150;
        ctx.beginPath();
        ctx.arc(x, y, 60, 0, Math.PI * 2);
        ctx.fillStyle = `hsl(${frame % 360}, 70%, 60%)`;
        ctx.fill();

        // Title text
        ctx.font = 'bold 48px Arial';
        ctx.fillStyle = '#ffffff';
        ctx.textAlign = 'center';
        ctx.fillText('Quick Support Test', 640, 200);

        // Subtitle
        ctx.font = '24px Arial';
        ctx.fillStyle = '#aaaaaa';
        ctx.fillText('Fake skærmdeling via Playwright', 640, 260);

        // Frame counter
        ctx.font = '18px monospace';
        ctx.fillStyle = '#4ade80';
        ctx.textAlign = 'left';
        ctx.fillText(`Frame: ${frame}`, 20, 700);

        // Timestamp
        ctx.textAlign = 'right';
        ctx.fillText(new Date().toLocaleTimeString('da-DK'), 1260, 700);

        requestAnimationFrame(draw);
      }
      draw();

      const stream = canvas.captureStream(30);
      return stream;
    };
  });

  // Open support page with token
  console.log('Åbner support-siden...');
  await page.goto(shareUrl, { waitUntil: 'networkidle' });

  // Wait for validation and "Del skærm" button
  console.log('Venter på session-validering...');
  await page.waitForSelector('#shareBtn:not([style*="display: none"])', { timeout: 15000 });
  console.log('Session valideret! Klikker "Del skærm"...');

  // Click share button
  await page.click('#shareBtn');
  console.log('Skærmdeling startet (falsk canvas stream)');

  // Wait for WebRTC connection
  console.log('Venter på WebRTC forbindelse...');
  try {
    await page.waitForSelector('#connectionState', { timeout: 5000 });
    // Wait for "Forbundet" text
    await page.waitForFunction(
      () => document.getElementById('connectionState')?.textContent === 'Forbundet',
      { timeout: 60000 }
    );
    console.log('');
    console.log('WebRTC FORBUNDET! Video streames nu.');
    console.log('Åbn dit dashboard for at se video i main preview.');
    console.log('');
    console.log('Tryk Ctrl+C for at stoppe...');
  } catch (e) {
    const stateText = await page.textContent('#connectionState').catch(() => 'unknown');
    console.log(`Connection state: ${stateText}`);
    console.log('Venter stadig på dashboard-forbindelse...');
    console.log('Åbn dashboard-linket ovenfor og vent...');
    console.log('Tryk Ctrl+C for at stoppe.');
  }

  // Keep running until Ctrl+C
  await new Promise((resolve) => {
    process.on('SIGINT', async () => {
      console.log('\nLukker ned...');
      await browser.close();
      // Cleanup session
      await supabase.from('support_sessions').delete().eq('id', sessionId);
      console.log('Session slettet. Færdig!');
      resolve();
    });
  });
}

main().catch(console.error);
