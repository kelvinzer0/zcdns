#!/bin/bash

# Change the form grid container
sed -i 's/grid grid-cols-1 sm:grid-cols-12 gap-3 sm:gap-4/grid grid-cols-2 sm:grid-cols-12 gap-3 sm:gap-4/g' src/components/dashboard/RecordManager.tsx

# Make Type take 1 col on mobile (2 cols total)
sed -i 's/<div className="sm:col-span-2">/<div className="col-span-1 sm:col-span-2">/g' src/components/dashboard/RecordManager.tsx

# Make Name take 2 cols on mobile (full width)
sed -i 's/<div className="sm:col-span-4">/<div className="col-span-2 sm:col-span-4">/g' src/components/dashboard/RecordManager.tsx

# Wait, TTL and Submit button? Let's check their classes.
