// Script to create the remote_users table in Supabase
const { createClient } = require('@supabase/supabase-js');

// Supabase project URL and anon key
const supabaseUrl = 'https://ptrtibzwokjcjjxvjpin.supabase.co';
const supabaseKey = 'sb_publishable_OwJ04BrJ6gZZ57fSeT-q5Q_mSsVaNia';

// Create a Supabase client
const supabase = createClient(supabaseUrl, supabaseKey);

async function createTable() {
  console.log('Creating remote_users table...');
  
  try {
    // First, let's try to create the table using SQL
    const { data, error } = await supabase.rpc('exec_sql', {
      sql: `
        CREATE TABLE IF NOT EXISTS remote_users (
          id UUID DEFAULT gen_random_uuid() PRIMARY KEY,
          created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
          username TEXT NOT NULL,
          email TEXT UNIQUE NOT NULL,
          last_login TIMESTAMP WITH TIME ZONE
        );
      `
    });

    if (error) {
      console.error('Error creating table:', error);
      console.log('Note: You may need to create the table manually in the Supabase dashboard.');
      return false;
    }

    console.log('Table created successfully!');
    return true;
  } catch (err) {
    console.error('Unexpected error:', err);
    console.log('Note: You may need to create the table manually in the Supabase dashboard.');
    return false;
  }
}

async function insertTestData() {
  console.log('Inserting test data...');
  
  try {
    const { data, error } = await supabase
      .from('remote_users')
      .insert([
        { username: 'testuser1', email: 'user1@example.com' },
        { username: 'testuser2', email: 'user2@example.com' }
      ]);

    if (error) {
      console.error('Error inserting test data:', error);
      return false;
    }

    console.log('Test data inserted successfully!');
    return true;
  } catch (err) {
    console.error('Unexpected error inserting data:', err);
    return false;
  }
}

async function testConnection() {
  console.log('Testing connection...');
  
  try {
    const { data, error } = await supabase
      .from('remote_users')
      .select('*')
      .limit(5);

    if (error) {
      console.error('Error querying table:', error);
      return false;
    }

    console.log('Successfully connected to Supabase!');
    console.log('Data:', data);
    return true;
  } catch (err) {
    console.error('Unexpected error:', err);
    return false;
  }
}

async function main() {
  console.log('Setting up Supabase table...\n');
  
  // Try to create the table
  const tableCreated = await createTable();
  
  if (tableCreated) {
    // Insert test data
    await insertTestData();
  }
  
  // Test the connection
  await testConnection();
  
  console.log('\nSetup complete!');
}

main();
