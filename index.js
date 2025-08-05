// Import the Supabase client
const { createClient } = require('@supabase/supabase-js');

// Supabase project URL and anon key (replace with your own values)
const supabaseUrl = 'YOUR_SUPABASE_PROJECT_URL';
const supabaseKey = 'YOUR_SUPABASE_ANON_KEY';

// Create a Supabase client
const supabase = createClient(supabaseUrl, supabaseKey);

// Example function to test the connection
async function testConnection() {
  try {
    // You'll need to replace 'your_table' with an actual table in your Supabase project
    const { data, error } = await supabase.from('your_table').select('*').limit(5);
    
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

// Uncomment to test the connection
// testConnection();

// Export the Supabase client for use in other files
module.exports = { supabase };
