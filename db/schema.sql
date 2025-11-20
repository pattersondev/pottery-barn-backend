-- Create products table
CREATE TABLE IF NOT EXISTS products (
  id SERIAL PRIMARY KEY,
  name VARCHAR(500) NOT NULL,
  price DECIMAL(10, 2),
  grade VARCHAR(50),
  image_url TEXT,
  product_url TEXT,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  UNIQUE(product_url)
);

-- Create index on product_url for faster lookups
CREATE INDEX IF NOT EXISTS idx_product_url ON products(product_url);

-- Create index on created_at for sorting
CREATE INDEX IF NOT EXISTS idx_created_at ON products(created_at);

