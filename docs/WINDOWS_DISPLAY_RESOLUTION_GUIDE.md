# Windows display resolution guide

Remote Desktop viser den opløsning agenten faktisk kan capture fra Windows.
Hvis dashboard/controller viser `640x480`, er det normalt ikke en WebRTC- eller
browserfejl. Det betyder typisk, at Windows-serveren kun har en lav
console-display mode tilgængelig.

## Typiske årsager

- Serveren er headless uden fysisk skærm eller dummy HDMI.
- RDP-sessionen har en høj opløsning, men console-sessionen har ikke.
- Windows er på login screen/Session 0, hvor GDI/console capture kun ser
  fallback-opløsningen.
- Display-driveren er Basic Display Adapter eller mangler et aktivt monitor EDID.

## Fix i prioriteret rækkefølge

1. Sæt en dummy HDMI/DisplayPort adapter i serveren.
2. Alternativt installer en virtuel display-driver på serveren.
3. Log ind på selve console-sessionen og sæt Windows Display Settings til den
   ønskede opløsning.
4. Undgå at stole på RDP-opløsningen som bevis; RDP kan have høj opløsning mens
   den lokale console stadig er `640x480`.
5. Genstart agenten efter display ændringen, hvis den ikke automatisk opdager
   ny opløsning.

## Hvordan det ses i appen

Dashboard og Wails-controller viser remote opløsning i viewer/statuslinjen.
Hvis opløsningen er `640x480` eller lavere på en akse, markeres den som lav
opløsning.
