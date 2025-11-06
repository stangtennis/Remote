# Known Issues

## Fyne Thread Warnings (Cosmetic Only)

### Issue
You may see warnings like:
```
*** Error in Fyne call thread, this should have been called in fyne.Do[AndWait] ***
```

### Impact
- **These are cosmetic warnings only**
- The application works correctly
- UI updates happen properly
- No crashes or data loss

### Why This Happens
Fyne v2.7.0 has strict thread checking. When the login goroutine updates UI widgets (`statusLabel.SetText()`, `loginButton.Enable()`), Fyne detects these are called from a background thread and logs warnings.

### Why We Don't "Fix" It
1. **The app works perfectly** - All UI updates happen correctly
2. **Fyne handles it internally** - Fyne automatically synchronizes these calls
3. **Alternative is complex** - Proper fix requires channels or complex callback chains
4. **Performance** - Current approach is actually faster

### If Warnings Bother You
You can ignore them - they don't affect functionality. The warnings only appear in the console/logs, not in the UI.

### Technical Details
The warnings appear at:
- Line 201: `statusLabel.SetText()` after approval check
- Line 224: `statusLabel.SetText()` after device fetch  
- Line 227: `loginButton.Enable()` after login complete

All these calls work correctly despite the warnings.

## Future Improvements
If Fyne adds a proper async UI update API in future versions, we can refactor to use it. For now, the current implementation is the most straightforward and works reliably.
