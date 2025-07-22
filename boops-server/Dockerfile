# Use Node.js official image
FROM node:22-alpine

# Create app directory
WORKDIR /usr/src/app

# Copy package.json and package-lock.json
COPY package*.json ./

# Install dependencies
RUN npm install --production

# Copy the rest of the application code
COPY . .

# Expose the port that the server runs on
EXPOSE 3000

# Start the server
CMD ["npm", "start"]
