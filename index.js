// Import the Supabase client
const { createClient } = require('@supabase/supabase-js');

// Supabase project URL and anon key
const supabaseUrl = 'https://ptrtibzwokjcjjxvjpin.supabase.co';
const supabaseKey = 'sb_publishable_OwJ04BrJ6gZZ57fSeT-q5Q_mSsVaNia';

// Create a Supabase client
const supabase = createClient(supabaseUrl, supabaseKey);

// Example function to test the connection
async function testConnection() {
  try {
    // Query the remote_users table
    const { data, error } = await supabase.from('remote_users').select('*').limit(5);
    
    if (error) {
      console.error('Error connecting to Supabase:', error);
    } else {
      console.log('Successfully connected to Supabase!');
      console.log('Data:', data);
    }
  } catch (err) {
    console.error('Unexpected error:', err);
  }
}

// Test the connection
testConnection();

// Export the Supabase client for use in other files
module.exports = { supabase };
