# Use the official Node.js image as a base
FROM node:22-alpine

# Set working directory
WORKDIR /app

# Copy package.json and package-lock.json
COPY package*.json ./

# Install dependencies
RUN npm install

# Copy the rest of the application code
COPY . .

# Build the Nuxt.js app
RUN npm run build

# Expose port 3000 for the development server
EXPOSE 3000

# Start the Nuxt.js app in development mode (for Docker)
CMD ["npm", "run", "preview"]
