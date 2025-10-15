# GiddyUp Documentation

**Professional documentation for the GiddyUp horse racing data platform**

## 🚀 Quick Navigation

### Essential Documentation (Read These)

1. **[📍 START HERE](00_START_HERE.md)** - Documentation index & quick links
2. **[👨‍💻 Developer Guide](01_DEVELOPER_GUIDE.md)** - Complete development guide  
3. **[📡 API Documentation](02_API_DOCUMENTATION.md)** - All API endpoints
4. **[🗄️ Database Guide](03_DATABASE_GUIDE.md)** - Database schema & queries
5. **[🎨 Frontend Guide](04_FRONTEND_GUIDE.md)** - UI integration guide
6. **[🚀 Deployment Guide](05_DEPLOYMENT_GUIDE.md)** - Production deployment

### Supplementary Documentation

- **[Auto-Update Feature](features/AUTO_UPDATE.md)** - Background data backfilling
- **[Auto-Update Logs](features/AUTO_UPDATE_EXAMPLE_LOGS.md)** - Log examples

---

## For Different Roles

### I'm a Frontend Developer

**Read these**:
1. [00_START_HERE.md](00_START_HERE.md) - Overview (5 min)
2. [04_FRONTEND_GUIDE.md](04_FRONTEND_GUIDE.md) - Integration patterns (20 min)
3. [02_API_DOCUMENTATION.md](02_API_DOCUMENTATION.md) - API reference (as needed)

**What you'll learn**:
- How to call API endpoints
- TypeScript type definitions
- Common UI patterns (search, profiles, racecards)
- Performance best practices
- Error handling

### I'm a Backend Developer

**Read these**:
1. [00_START_HERE.md](00_START_HERE.md) - Overview (5 min)
2. [01_DEVELOPER_GUIDE.md](01_DEVELOPER_GUIDE.md) - Development workflow (30 min)
3. [03_DATABASE_GUIDE.md](03_DATABASE_GUIDE.md) - Database schema (as needed)

**What you'll learn**:
- Project structure
- How to add endpoints
- Database queries
- Testing procedures
- Debugging techniques

### I'm a DevOps Engineer

**Read these**:
1. [00_START_HERE.md](00_START_HERE.md) - Overview (5 min)
2. [05_DEPLOYMENT_GUIDE.md](05_DEPLOYMENT_GUIDE.md) - Deployment (30 min)
3. [features/AUTO_UPDATE.md](features/AUTO_UPDATE.md) - Auto-update service (10 min)

**What you'll learn**:
- How to deploy to production
- Environment configuration
- Monitoring & logging
- Backup/restore procedures
- Troubleshooting

---

## Quick Start (5 Minutes)

```bash
# 1. Start database
cd postgres && docker-compose up -d

# 2. Restore data
docker exec -i horse_racing psql -U postgres -d horse_db < db_backup.sql

# 3. Start API
cd backend-api && ./bin/api

# 4. Test
curl http://localhost:8000/health
curl "http://localhost:8000/api/v1/courses" | jq
```

**Done!** API is running on http://localhost:8000

---

## Documentation Status

| Document | Status | Audience | Reading Time |
|----------|--------|----------|--------------|
| 00_START_HERE | ✅ Current | All | 5 min |
| 01_DEVELOPER_GUIDE | ✅ Current | Backend | 30 min |
| 02_API_DOCUMENTATION | ✅ Current | All Devs | Reference |
| 03_DATABASE_GUIDE | ✅ Current | Backend/Data | 20 min |
| 04_FRONTEND_GUIDE | ✅ Current | Frontend | 20 min |
| 05_DEPLOYMENT_GUIDE | ✅ Current | DevOps | 30 min |

**All documentation is current as of October 15, 2025**

---

## Archive

Historical documentation (50+ files) moved to `archive/` folder:
- Project status updates
- Implementation summaries
- Test results
- Legacy guides

**These are preserved for reference but not maintained.**

---

**Last Updated**: October 15, 2025  
**Version**: 1.0.0  
**Status**: ✅ Production Ready  
**Test Coverage**: 97% (32/33 passing)
